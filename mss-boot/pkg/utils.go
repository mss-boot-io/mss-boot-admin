package pkg

import (
	"os"
	"reflect"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/spf13/cast"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	mgm "github.com/kamva/mgm/v3"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/schema"
)

const (
	// TrafficKey traffic key
	TrafficKey = "X-Request-ID"
	// LoggerKey logger key
	LoggerKey = "_go-admin-logger-request"
)

// CompareHashAndPassword compare hash and password
func CompareHashAndPassword(hash string, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return false, err
	}
	return true, nil
}

// GenerateMsgIDFromContext 生成msgID
func GenerateMsgIDFromContext(c *gin.Context) string {
	requestID := c.GetHeader(TrafficKey)
	if requestID == "" {
		requestID = uuid.New().String()
		c.Header(TrafficKey, requestID)
	}
	return requestID
}

// ModelDeepCopy model deep copy
func ModelDeepCopy(m mgm.Model) mgm.Model {
	return reflect.New(reflect.TypeOf(m).Elem()).Interface().(mgm.Model)
}

// TablerDeepCopy model deep copy
func TablerDeepCopy(m schema.Tabler) schema.Tabler {
	return reflect.New(reflect.TypeOf(m).Elem()).Interface().(schema.Tabler)
}

// DeepCopy deep copy
func DeepCopy(d any) any {
	return reflect.New(reflect.TypeOf(d).Elem()).Interface()
}

// BuildMap build map
func BuildMap(keys []string, value string, dataType enum.DataType) map[string]any {
	data := make(map[string]any)
	if len(keys) > 1 {
		data[keys[0]] = BuildMap(keys[1:], value, dataType)
	} else {
		var v any
		switch dataType {
		case enum.DataTypeInt:
			v, _ = cast.ToIntE(value)
		case enum.DataTypeFloat:
			v, _ = cast.ToFloat64E(value)
		case enum.DataTypeBool:
			v, _ = cast.ToBoolE(value)
		default:
			v = value
		}
		return map[string]any{keys[0]: v}
	}
	return data
}

// MergeMapsDepth deep merge multi map
func MergeMapsDepth(ms ...map[string]any) map[string]any {
	data := make(map[string]any)
	for i := range ms {
		data = MergeMapDepth(data, ms[i])
	}
	return data
}

// MergeMapDepth deep merge map
func MergeMapDepth(m1, m2 map[string]any) map[string]any {
	for k := range m2 {
		if v, ok := m1[k]; ok {
			if m, ok := v.(map[string]any); ok {
				m1[k] = MergeMapDepth(m, m2[k].(map[string]any))
			} else {
				m1[k] = m2[k]
			}
		} else {
			m1[k] = m2[k]
		}
	}
	return m1
}

// MergeMap merge map
func MergeMap(m1, m2 map[string]any) map[string]any {
	for k := range m2 {
		m1[k] = m2[k]
	}
	return m1
}

// SupportMultiTenant support multi tenant
func SupportMultiTenant(data any) bool {
	return supportColumn(data, "tenantID", "tenant_id")
}

func SupportCreator(data any) bool {
	return supportColumn(data, "creatorID", "creator_id")
}

func GetCreatorField() string {
	return "creator_id"
}

func SetCreator(data any, id string) {
	SetValue(data, "creatorID", id)
}

func supportColumn(data any, fields ...string) bool {
	typeOf := reflect.TypeOf(data)
	valueOf := reflect.ValueOf(data)
	if typeOf.Kind() == reflect.Ptr {
		typeOf = typeOf.Elem()
		valueOf = valueOf.Elem()
	}

	var exist bool
	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)
		if field.Type.Kind() == reflect.Struct {
			exist = supportColumn(valueOf.Field(i).Interface(), fields...)
		}
		if field.Type.Kind() == reflect.Ptr {
			continue
		}
		for j := range fields {
			exist = exist || strings.EqualFold(field.Name, fields[j])
			if exist {
				break
			}
		}
		if exist {
			break
		}
	}
	return exist
}

func SetValue(data any, key string, value any) {
	typeOf := reflect.TypeOf(data)
	valueOf := reflect.ValueOf(data)
	if typeOf.Kind() == reflect.Ptr {
		typeOf = typeOf.Elem()
		valueOf = valueOf.Elem()
	}
	key = strings.ToLower(key)
	for i := 0; i < typeOf.NumField(); i++ {
		field := typeOf.Field(i)
		if field.Type.Kind() == reflect.Ptr {
			continue
		}
		if field.Type.Kind() == reflect.Struct {
			SetValue(valueOf.Field(i).Interface(), key, value)
			continue
		}
		if strings.EqualFold(field.Name, key) {
			v := reflect.ValueOf(value)
			valueOf.FieldByName(field.Name).Set(v)
		}
	}
}

// ParseEnvTemplate 替换环境变量模板
func ParseEnvTemplate(t string) string {
	var err error
	temp := template.New("env")
	temp, err = temp.Parse(t)
	if err != nil {
		return t
	}
	tree, err := parse.Parse("env", t, "{{", "}}")
	if err != nil {
		return t
	}
	vars := make(map[string]string)
	for _, v := range getParseKeys(tree["env"].Root) {
		vars[v] = os.Getenv(v)
	}
	var buf strings.Builder
	err = temp.Execute(&buf, vars)
	if err != nil {
		return t
	}
	return buf.String()
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

// GetStage get stage
func GetStage() string {
	stage := os.Getenv("stage")
	if stage == "" {
		stage = os.Getenv("STAGE")
	}
	if stage == "" {
		stage = "local"
	}
	return stage
}

func GetProjectName() string {
	project := os.Getenv("project_name")
	if project == "" {
		project = os.Getenv("PROJECT_NAME")
	}
	if project == "" {
		project = "mss-boot-io"
	}
	return project
}

func GetNodeName() string {
	node := os.Getenv("node_name")
	if node == "" {
		node = os.Getenv("NODE_NAME")
	}
	if node == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return "unknown"
		}
		node = hostname
	}
	return node
}
