package config

/*
 * @Author: lwnmengjing
 * @Date: 2021/5/18 12:31 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/5/18 12:31 下午
 */

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/template"
	"text/template/parse"

	"gopkg.in/yaml.v3"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
	"github.com/mss-boot-io/mss-boot/pkg/config/source/appconfig"
	"github.com/mss-boot-io/mss-boot/pkg/config/source/configmap"
	sourceConsul "github.com/mss-boot-io/mss-boot/pkg/config/source/consul"
	sourceFS "github.com/mss-boot-io/mss-boot/pkg/config/source/fs"
	"github.com/mss-boot-io/mss-boot/pkg/config/source/gorm"
	sourceLocal "github.com/mss-boot-io/mss-boot/pkg/config/source/local"
	"github.com/mss-boot-io/mss-boot/pkg/config/source/mgdb"
	sourceS3 "github.com/mss-boot-io/mss-boot/pkg/config/source/s3"
)

// Init 初始化配置
func Init(cfg source.Entity, options ...source.Option) (err error) {
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
		s := &Storage{
			Type:            ProviderType(os.Getenv("s3_provider")),
			SigningMethod:   os.Getenv("s3_signing_method"),
			Region:          os.Getenv("s3_region"),
			Bucket:          os.Getenv("s3_bucket"),
			Endpoint:        os.Getenv("s3_endpoint"),
			AccessKeyID:     os.Getenv("s3_access_key_id"),
			SecretAccessKey: os.Getenv("s3_secret_access_key"),
		}
		s.Init()
		options = append(options,
			source.WithBucket(s.Bucket), source.WithClient(s.GetClient()))
		f, err = sourceS3.New(options...)
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
	default:
		f, err = sourceLocal.New(options...)
	}
	if err != nil {
		return err
	}
	if f == nil {
		return fmt.Errorf("source not found")
	}

	var rb []byte
	rb, err = f.ReadFile(opts.Name)
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

	rb, err = f.ReadFile(fmt.Sprintf("%s-%s", opts.Name, stage))
	if err == nil {
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
		}
	}
	// postfix hook
	if opts.PostfixHook != nil {
		err = unm(rb, opts.PostfixHook)
		if err != nil {
			slog.Error(err.Error())
			return err
		}
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
