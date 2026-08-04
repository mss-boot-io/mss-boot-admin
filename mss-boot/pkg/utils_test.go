package pkg

import (
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	mgm "github.com/kamva/mgm/v3"
	"golang.org/x/crypto/bcrypt"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
)

type testTabler struct{}

func (*testTabler) TableName() string { return "test_table" }

type testMGMModel struct {
	mgm.DefaultModel `bson:",inline"`
}

type OwnershipFields struct {
	TenantID  string `json:"tenantID" gorm:"column:tenant_id"`
	CreatorID string `json:"creatorID" gorm:"column:creator_id"`
}

type ownershipModel struct {
	*OwnershipFields
}

func TestCompareHashAndPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	matched, err := CompareHashAndPassword(string(hash), "correct-password")
	if err != nil || !matched {
		t.Fatalf("valid password matched=%t error=%v", matched, err)
	}
	matched, err = CompareHashAndPassword(string(hash), "wrong-password")
	if err == nil || matched {
		t.Fatalf("invalid password matched=%t error=%v", matched, err)
	}
	if matched, err = CompareHashAndPassword("not-a-hash", "password"); err == nil || matched {
		t.Fatalf("malformed hash matched=%t error=%v", matched, err)
	}
}

func TestGenerateMsgIDFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	context.Request.Header.Set(TrafficKey, "existing-request")
	if got := GenerateMsgIDFromContext(context); got != "existing-request" {
		t.Fatalf("existing request ID = %q", got)
	}

	context.Request.Header.Del(TrafficKey)
	generated := GenerateMsgIDFromContext(context)
	if generated == "" || recorder.Header().Get(TrafficKey) != generated {
		t.Fatalf("generated request ID = %q, response header = %q", generated, recorder.Header().Get(TrafficKey))
	}
	if generated = GenerateMsgIDFromContext(nil); generated == "" {
		t.Fatal("nil context returned empty request ID")
	}
}

func TestPointerCopyHelpers(t *testing.T) {
	if clone, ok := DeepCopy(&OwnershipFields{}).(*OwnershipFields); !ok || clone == nil {
		t.Fatalf("DeepCopy result = %#v", clone)
	}
	if DeepCopy(OwnershipFields{}) != nil || DeepCopy(nil) != nil {
		t.Fatal("DeepCopy must reject non-pointers and nil")
	}
	if clone := TablerDeepCopy(&testTabler{}); clone == nil || clone.TableName() != "test_table" {
		t.Fatalf("TablerDeepCopy result = %#v", clone)
	}
	if TablerDeepCopy(nil) != nil {
		t.Fatal("TablerDeepCopy(nil) must return nil")
	}
	if clone := ModelDeepCopy(&testMGMModel{}); clone == nil {
		t.Fatal("ModelDeepCopy returned nil")
	}
	if ModelDeepCopy(nil) != nil {
		t.Fatal("ModelDeepCopy(nil) must return nil")
	}
}

func TestBuildAndMergeMaps(t *testing.T) {
	if got := BuildMap(nil, "value", enum.DataTypeString); len(got) != 0 {
		t.Fatalf("empty keys map = %#v", got)
	}
	cases := []struct {
		dataType enum.DataType
		value    string
		want     any
	}{
		{enum.DataTypeString, "value", "value"},
		{enum.DataTypeInt, "42", 42},
		{enum.DataTypeFloat, "3.5", 3.5},
		{enum.DataTypeBool, "true", true},
	}
	for _, test := range cases {
		got := BuildMap([]string{"outer", "inner"}, test.value, test.dataType)
		inner := got["outer"].(map[string]any)["inner"]
		if !reflect.DeepEqual(inner, test.want) {
			t.Fatalf("BuildMap(%s) = %#v, want %#v", test.dataType, inner, test.want)
		}
	}

	merged := MergeMapsDepth(
		map[string]any{"database": map[string]any{"host": "old", "port": 3306}},
		map[string]any{"database": map[string]any{"host": "new"}, "debug": true},
	)
	database := merged["database"].(map[string]any)
	if database["host"] != "new" || database["port"] != 3306 || merged["debug"] != true {
		t.Fatalf("deep merge = %#v", merged)
	}
	if got := MergeMapDepth(map[string]any{"value": map[string]any{"nested": true}}, map[string]any{"value": "replacement"}); got["value"] != "replacement" {
		t.Fatalf("map-to-scalar replacement = %#v", got)
	}
	if got := MergeMap(nil, map[string]any{"key": "value"}); got["key"] != "value" {
		t.Fatalf("shallow merge = %#v", got)
	}
}

func TestOwnershipReflectionHelpers(t *testing.T) {
	model := &ownershipModel{}
	if !SupportMultiTenant(model) || !SupportCreator(model) {
		t.Fatalf("ownership support tenant=%t creator=%t", SupportMultiTenant(model), SupportCreator(model))
	}
	if GetCreatorField() != "creator_id" {
		t.Fatalf("creator field = %q", GetCreatorField())
	}
	SetCreator(model, "creator-1")
	if model.OwnershipFields == nil || model.CreatorID != "creator-1" {
		t.Fatalf("creator was not assigned: %#v", model)
	}
	SetValue(model, "tenant_id", "tenant-1")
	if model.TenantID != "tenant-1" {
		t.Fatalf("tenant was not assigned: %#v", model)
	}
	SetValue(model, "missing", "ignored")
	SetValue(nil, "creatorID", "ignored")
	if SupportCreator(struct{}{}) || SupportMultiTenant(nil) {
		t.Fatal("unsupported values reported ownership fields")
	}
}

func TestEnvironmentTemplateAndIdentityHelpers(t *testing.T) {
	environment := map[string]string{
		"DIRECT_VALUE": "direct",
		"NESTED_VALUE": "nested",
		"stage":        "dev",
		"STAGE":        "prod",
		"project_name": "lower-project",
		"PROJECT_NAME": "upper-project",
		"node_name":    "lower-node",
		"NODE_NAME":    "upper-node",
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}

	if got := ParseEnvTemplate("{{.DIRECT_VALUE}}/{{.Env.NESTED_VALUE}}"); got != "direct/nested" {
		t.Fatalf("environment template = %q", got)
	}
	if got := ParseEnvTemplate("{{.MISSING_VALUE}}"); got != "" {
		t.Fatalf("missing environment template = %q", got)
	}
	invalid := "{{"
	if got := ParseEnvTemplate(invalid); got != invalid {
		t.Fatalf("invalid template = %q", got)
	}
	if GetStage() != "dev" || GetProjectName() != "lower-project" || GetNodeName() != "lower-node" {
		t.Fatalf("identity values stage=%q project=%q node=%q", GetStage(), GetProjectName(), GetNodeName())
	}

	_ = os.Unsetenv("stage")
	_ = os.Unsetenv("project_name")
	_ = os.Unsetenv("node_name")
	if GetStage() != "prod" || GetProjectName() != "upper-project" || GetNodeName() != "upper-node" {
		t.Fatalf("uppercase identity fallback stage=%q project=%q node=%q", GetStage(), GetProjectName(), GetNodeName())
	}

	_ = os.Unsetenv("STAGE")
	_ = os.Unsetenv("PROJECT_NAME")
	_ = os.Unsetenv("NODE_NAME")
	if GetStage() != "local" || GetProjectName() != "mss-boot-io" || strings.TrimSpace(GetNodeName()) == "" {
		t.Fatalf("default identity stage=%q project=%q node=%q", GetStage(), GetProjectName(), GetNodeName())
	}
}
