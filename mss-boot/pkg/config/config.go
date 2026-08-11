package config

/*
 * @Author: lwnmengjing
 * @Date: 2021/5/18 12:31 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/5/18 12:31 下午
 */

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"text/template/parse"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gopkg.in/yaml.v3"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source/appconfig"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source/configmap"
	sourceConsul "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source/consul"
	sourceFS "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source/fs"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source/gorm"
	sourceLocal "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source/local"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source/mgdb"
	sourceS3 "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/source/s3"
)

// Init 初始化配置
func Init(cfg source.Entity, options ...source.Option) error {
	return InitContext(context.Background(), cfg, options...)
}

// InitContext initializes configuration with caller-owned cancellation. An S3
// configuration source gets a bootstrap-only handle that is never shared with
// application object storage.
func InitContext(ctx context.Context, cfg source.Entity, options ...source.Option) (err error) {
	if ctx == nil {
		return errors.New("config initialization context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	opts := source.DefaultOptions()
	for _, opt := range options {
		opt(opts)
	}
	stage := pkg.GetStage()
	var f source.Sourcer
	switch opts.Provider {
	case source.FS:
		f, err = sourceFS.New(options...)
	case source.S3:
		bootstrap, bootstrapErr := s3BootstrapStorageFromEnvironment()
		if bootstrapErr != nil {
			return bootstrapErr
		}
		profile, normalizeErr := bootstrap.Normalize(ctx, EnvSecretResolver{})
		if normalizeErr != nil {
			return fmt.Errorf("normalize S3 configuration source: %w", normalizeErr)
		}
		handle, buildErr := profile.Build(ctx)
		if buildErr != nil {
			return fmt.Errorf("build S3 configuration source: %w", buildErr)
		}
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			err = errors.Join(err, handle.Close(closeCtx))
		}()
		err = handle.Use(ctx, func(_ *StorageProfile, client *s3.Client) error {
			s3Options := append(
				append([]source.Option(nil), options...),
				source.WithBucket(profile.bucket),
				source.WithClient(client),
				source.WithContext(ctx),
			)
			bootstrapSource, sourceErr := sourceS3.New(s3Options...)
			if sourceErr != nil {
				return sourceErr
			}
			return initFromSource(cfg, opts, bootstrapSource, stage)
		})
		return err
	case source.MGDB:
		f, err = mgdb.New(options...)
	case source.GORM:
		f, err = gorm.New(options...)
	case source.ConfigMap:
		options = append([]source.Option{source.WithNamespace(strings.ToLower(stage))}, options...)
		f, err = configmap.New(options...)
	case source.Consul:
		f, err = sourceConsul.New(options...)
	case source.APPConfig:
		f, err = appconfig.New(options...)
	case source.Local, "":
		f, err = sourceLocal.New(options...)
	default:
		return fmt.Errorf("config source provider %q is not supported", opts.Provider)
	}
	if err != nil {
		return err
	}
	if f == nil {
		return fmt.Errorf("source not found")
	}
	return initFromSource(cfg, opts, f, stage)
}

func initFromSource(cfg source.Entity, opts *source.Options, f source.Sourcer, stage string) (err error) {
	rb, err := f.ReadFile(opts.Name)
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	var unm func([]byte, any) error
	switch f.GetExtend() {
	case source.SchemeYaml, source.SchemeYml:
		unm = yaml.Unmarshal
	case source.SchemeJSOM:
		unm = json.Unmarshal
	}
	if unm == nil {
		return fmt.Errorf("configuration source extension %q is unsupported", f.GetExtend())
	}
	if opts.PrefixHook != nil {
		err = unm(rb, opts.PrefixHook)
		if err != nil {
			slog.Error(err.Error())
			return err
		}
		opts.PrefixHook.Init()
	}
	rb, err = parseTemplateWithEnv(rb)
	if err != nil {
		return err
	}
	err = unm(rb, cfg)
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	// postfix hook
	if opts.PostfixHook != nil {
		err = unm(rb, opts.PostfixHook)
		if err != nil {
			slog.Error(err.Error())
			return err
		}
	}

	overlay, overlayErr := f.ReadFile(fmt.Sprintf("%s-%s", opts.Name, stage))
	switch {
	case overlayErr == nil:
		if opts.PrefixHook != nil {
			err = unm(overlay, opts.PrefixHook)
			if err != nil {
				slog.Error(err.Error())
				return err
			}
			opts.PrefixHook.Init()
		}
		overlay, err = parseTemplateWithEnv(overlay)
		if err != nil {
			return err
		}
		// Validate the overlay against a fresh value of the same concrete type
		// before mutating cfg. Startup still aborts on any decode error, and the
		// common type-mismatch path cannot leave a partially applied snapshot.
		if err = validateConfigOverlay(overlay, cfg, unm); err != nil {
			return err
		}
		err = unm(overlay, cfg)
		if err != nil {
			slog.Error(err.Error())
			return err
		}
		if opts.PostfixHook != nil {
			err = unm(overlay, opts.PostfixHook)
			if err != nil {
				slog.Error(err.Error())
				return err
			}
		}
	case errors.Is(overlayErr, fs.ErrNotExist):
		// A stage overlay is optional only when the source explicitly reports it
		// missing. AccessDenied, network failures, and other provider errors are
		// startup failures rather than permission to run the base snapshot.
	case overlayErr != nil:
		return fmt.Errorf("read stage configuration overlay: %w", overlayErr)
	default:
		return errors.New("read stage configuration overlay: unknown failure")
	}

	if !opts.Watch {
		return nil
	}
	if opts.PostfixHook != nil {
		err = f.Watch(opts.PostfixHook, unm)
		if err != nil {
			slog.Warn("watch custom config failed", "err", err)
			// ignore error
			err = nil
		}
	}
	return f.Watch(cfg, unm)
}

func validateConfigOverlay(data []byte, cfg source.Entity, unm func([]byte, any) error) error {
	typeOfConfig := reflect.TypeOf(cfg)
	if typeOfConfig == nil || typeOfConfig.Kind() != reflect.Pointer || reflect.ValueOf(cfg).IsNil() ||
		typeOfConfig.Elem().Kind() != reflect.Struct {
		return errors.New("configuration entity must be a non-nil pointer to a struct")
	}
	scratch := reflect.New(typeOfConfig.Elem()).Interface()
	if err := unm(data, scratch); err != nil {
		return fmt.Errorf("validate stage configuration overlay: %w", err)
	}
	return nil
}

func s3BootstrapStorageFromEnvironment() (Storage, error) {
	usePathStyle := false
	if raw := storageEnvironment("s3_use_path_style"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return Storage{}, storageConfigError("s3.usePathStyle", "must be a boolean")
		}
		usePathStyle = parsed
	}
	allowInsecureHTTP := false
	if raw := storageEnvironment("s3_tls_allow_insecure_http"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return Storage{}, storageConfigError("s3.tls.allowInsecureHTTP", "must be a boolean")
		}
		allowInsecureHTTP = parsed
	}
	credentials := S3CredentialsConfig{}
	switch strings.ToLower(strings.TrimSpace(storageEnvironment("s3_credential_source"))) {
	case "default-chain":
		credentials.DefaultChain = &DefaultChainCredentials{}
	case "static":
		credentials.Static = &StaticCredentialRefs{
			AccessKeyRef:    SecretRef(storageEnvironment("s3_access_key_ref")),
			SecretKeyRef:    SecretRef(storageEnvironment("s3_secret_key_ref")),
			SessionTokenRef: SecretRef(storageEnvironment("s3_session_token_ref")),
		}
	default:
		return Storage{}, storageConfigError("s3.credentials", "s3_credential_source must be default-chain or static")
	}
	return Storage{S3: &S3StorageConfig{
		Endpoint:     storageEnvironment("s3_endpoint"),
		Region:       storageEnvironment("s3_region"),
		Bucket:       storageEnvironment("s3_bucket"),
		UsePathStyle: usePathStyle,
		TLS: S3TLSConfig{
			CARef:                SecretRef(storageEnvironment("s3_tls_ca_ref")),
			ClientCertificateRef: SecretRef(storageEnvironment("s3_tls_client_certificate_ref")),
			ClientKeyRef:         SecretRef(storageEnvironment("s3_tls_client_key_ref")),
			AllowInsecureHTTP:    allowInsecureHTTP,
		},
		Credentials: credentials,
	}}, nil
}

func storageEnvironment(name string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return os.Getenv(strings.ToUpper(name))
}

func parseTemplateWithEnv(rb []byte) ([]byte, error) {
	t, err := template.New("env").Parse(string(rb))
	if err != nil {
		return nil, err
	}
	tree, err := parse.Parse("env", string(rb), "{{", "}}")
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	data := getValueFromEnv(getParseKeys(tree["env"].Root))
	err = t.Execute(&buffer, data)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func getValueFromEnv(keys []string) any {
	env := make(map[string]string)
	for i := range keys {
		if !strings.Contains(strings.ToLower(keys[i]), "env.") {
			continue
		}
		keyArr := strings.Split(keys[i], ".")
		if len(keyArr) > 1 {
			keyArr = keyArr[1:]
		}
		key := strings.Join(keyArr, ".")
		var exist bool
		env[key], exist = os.LookupEnv(key)
		if exist {
			continue
		}
		env[key], exist = os.LookupEnv(strings.ToUpper(key))
		if exist {
			continue
		}
		env[key], exist = os.LookupEnv(strings.ToLower(key))
		if exist {
			continue
		}
		env[key] = ""
	}
	return map[string]any{
		"Env": env,
	}
}

// getParseKeys get parse keys from template text
func getParseKeys(nodes *parse.ListNode) []string {
	keys := make([]string, 0)
	if nodes == nil {
		return keys
	}
	for a := range nodes.Nodes {
		if actionNode, ok := nodes.Nodes[a].(*parse.ActionNode); ok {
			if actionNode == nil || actionNode.Pipe == nil {
				continue
			}
			for b := range actionNode.Pipe.Cmds {
				if strings.Index(actionNode.Pipe.Cmds[b].String(), ".") == 0 {
					keys = append(keys, actionNode.Pipe.Cmds[b].String()[1:])
				}
			}
		}
	}
	return keys
}
