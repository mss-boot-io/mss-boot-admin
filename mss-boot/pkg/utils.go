package pkg

import (
	"os"
	"reflect"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	mgm "github.com/kamva/mgm/v3"
	"github.com/spf13/cast"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/schema"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
)

const (
	TrafficKey = "X-Request-ID"
	LoggerKey  = "_go-admin-logger-request"
)

func CompareHashAndPassword(hash string, password string) (bool, error) {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return false, err
	}
	return true, nil
}

func GenerateMsgIDFromContext(c *gin.Context) string {
	if c == nil {
		return uuid.New().String()
	}
	requestID := c.GetHeader(TrafficKey)
	if requestID == "" {
		requestID = uuid.New().String()
		c.Header(TrafficKey, requestID)
	}
	return requestID
}

func ModelDeepCopy(model mgm.Model) mgm.Model {
	clone := clonePointer(model)
	if clone == nil {
		return nil
	}
	result, _ := clone.(mgm.Model)
	return result
}

func TablerDeepCopy(model schema.Tabler) schema.Tabler {
	clone := clonePointer(model)
	if clone == nil {
		return nil
	}
	result, _ := clone.(schema.Tabler)
	return result
}

func DeepCopy(value any) any {
	return clonePointer(value)
}

func clonePointer(value any) any {
	if value == nil {
		return nil
	}
	typeOf := reflect.TypeOf(value)
	if typeOf.Kind() != reflect.Ptr {
		return nil
	}
	return reflect.New(typeOf.Elem()).Interface()
}

func BuildMap(keys []string, value string, dataType enum.DataType) map[string]any {
	if len(keys) == 0 {
		return map[string]any{}
	}
	key := keys[0]
	if len(keys) > 1 {
		return map[string]any{key: BuildMap(keys[1:], value, dataType)}
	}

	var converted any
	switch dataType {
	case enum.DataTypeInt:
		converted, _ = cast.ToIntE(value)
	case enum.DataTypeFloat:
		converted, _ = cast.ToFloat64E(value)
	case enum.DataTypeBool:
		converted, _ = cast.ToBoolE(value)
	default:
		converted = value
	}
	return map[string]any{key: converted}
}

func MergeMapsDepth(maps ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, current := range maps {
		result = MergeMapDepth(result, current)
	}
	return result
}

func MergeMapDepth(destination, source map[string]any) map[string]any {
	if destination == nil {
		destination = make(map[string]any)
	}
	for key, sourceValue := range source {
		destinationValue, exists := destination[key]
		destinationMap, destinationIsMap := destinationValue.(map[string]any)
		sourceMap, sourceIsMap := sourceValue.(map[string]any)
		if exists && destinationIsMap && sourceIsMap {
			destination[key] = MergeMapDepth(destinationMap, sourceMap)
			continue
		}
		destination[key] = sourceValue
	}
	return destination
}

func MergeMap(destination, source map[string]any) map[string]any {
	if destination == nil {
		destination = make(map[string]any)
	}
	for key, value := range source {
		destination[key] = value
	}
	return destination
}

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
	return typeSupportsColumn(reflect.TypeOf(data), fields)
}

func typeSupportsColumn(typeOf reflect.Type, fields []string) bool {
	for typeOf != nil && typeOf.Kind() == reflect.Ptr {
		typeOf = typeOf.Elem()
	}
	if typeOf == nil || typeOf.Kind() != reflect.Struct {
		return false
	}
	for index := 0; index < typeOf.NumField(); index++ {
		field := typeOf.Field(index)
		for _, candidate := range fields {
			if strings.EqualFold(field.Name, candidate) ||
				strings.EqualFold(parseTagName(field.Tag.Get("gorm"), "column"), candidate) ||
				strings.EqualFold(parseJSONName(field.Tag.Get("json")), candidate) {
				return true
			}
		}
		if field.Anonymous || field.Type.Kind() == reflect.Struct ||
			(field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Struct) {
			if typeSupportsColumn(field.Type, fields) {
				return true
			}
		}
	}
	return false
}

func parseTagName(tag, key string) string {
	for _, part := range strings.Split(tag, ";") {
		part = strings.TrimSpace(part)
		prefix := key + ":"
		if strings.HasPrefix(part, prefix) {
			return strings.TrimPrefix(part, prefix)
		}
	}
	return ""
}

func parseJSONName(tag string) string {
	return strings.Split(tag, ",")[0]
}

func SetValue(data any, key string, value any) {
	setReflectValue(reflect.ValueOf(data), key, reflect.ValueOf(value))
}

func setReflectValue(current reflect.Value, key string, value reflect.Value) bool {
	if !current.IsValid() {
		return false
	}
	for current.Kind() == reflect.Ptr {
		if current.IsNil() {
			if !current.CanSet() {
				return false
			}
			current.Set(reflect.New(current.Type().Elem()))
		}
		current = current.Elem()
	}
	if current.Kind() != reflect.Struct {
		return false
	}

	for index := 0; index < current.NumField(); index++ {
		fieldValue := current.Field(index)
		fieldType := current.Type().Field(index)
		if strings.EqualFold(fieldType.Name, key) ||
			strings.EqualFold(parseJSONName(fieldType.Tag.Get("json")), key) ||
			strings.EqualFold(parseTagName(fieldType.Tag.Get("gorm"), "column"), key) {
			if assignReflectValue(fieldValue, value) {
				return true
			}
		}
		if fieldType.Anonymous || fieldValue.Kind() == reflect.Struct || fieldValue.Kind() == reflect.Ptr {
			if setReflectValue(fieldValue, key, value) {
				return true
			}
		}
	}
	return false
}

func assignReflectValue(destination, source reflect.Value) bool {
	if !destination.IsValid() || !destination.CanSet() || !source.IsValid() {
		return false
	}
	if source.Type().AssignableTo(destination.Type()) {
		destination.Set(source)
		return true
	}
	if source.Type().ConvertibleTo(destination.Type()) {
		destination.Set(source.Convert(destination.Type()))
		return true
	}
	return false
}

func ParseEnvTemplate(text string) string {
	tmpl, err := template.New("env").Option("missingkey=error").Parse(text)
	if err != nil {
		return text
	}
	tree, err := parse.Parse("env", text, "{{", "}}")
	if err != nil || tree["env"] == nil {
		return text
	}

	values := make(map[string]any)
	environment := make(map[string]string)
	values["Env"] = environment
	for _, key := range getParseKeys(tree["env"].Root) {
		if strings.HasPrefix(key, "Env.") {
			environmentKey := strings.TrimPrefix(key, "Env.")
			environment[environmentKey] = os.Getenv(environmentKey)
			continue
		}
		values[key] = os.Getenv(key)
	}

	var builder strings.Builder
	if err = tmpl.Execute(&builder, values); err != nil {
		return text
	}
	return builder.String()
}

func getParseKeys(nodes *parse.ListNode) []string {
	keys := make([]string, 0)
	if nodes == nil {
		return keys
	}
	for _, node := range nodes.Nodes {
		actionNode, ok := node.(*parse.ActionNode)
		if !ok || actionNode == nil || actionNode.Pipe == nil {
			continue
		}
		for _, command := range actionNode.Pipe.Cmds {
			value := command.String()
			if strings.HasPrefix(value, ".") {
				keys = append(keys, strings.TrimPrefix(value, "."))
			}
		}
	}
	return keys
}

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
