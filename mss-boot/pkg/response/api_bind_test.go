package response

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type mixedBindRequest struct {
	ID   string `uri:"id" binding:"required"`
	Name string `json:"name" binding:"required"`
}

type queryAndBodyRequest struct {
	Filter string `query:"filter" form:"filter"`
	Name   string `query:"name" form:"name" json:"name"`
}

func TestBindCombinesURIAndBodyBeforeFinalValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/records/42", bytes.NewBufferString(`{"name":"acme"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: "42"}}

	var request mixedBindRequest
	api := Make(context).Bind(&request)
	if api.Error != nil {
		t.Fatalf("bind failed: %v", api.Error)
	}
	if request.ID != "42" || request.Name != "acme" {
		t.Fatalf("unexpected bind result: %+v", request)
	}
}

func TestBindGETSkipsRequestBodyBindings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/records?name=query", strings.NewReader("{invalid-json"))
	context.Request.Header.Set("Content-Type", "application/json")

	var request queryAndBodyRequest
	api := Make(context).Bind(&request)
	if api.Error != nil {
		t.Fatalf("GET bind unexpectedly read the body: %v", api.Error)
	}
	if request.Name != "query" {
		t.Fatalf("query value was not bound: %+v", request)
	}
}

func TestBindCombinesQueryAndJSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/records?filter=active", bytes.NewBufferString(`{"name":"acme"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	var request queryAndBodyRequest
	api := Make(context).Bind(&request)
	if api.Error != nil {
		t.Fatalf("bind failed: %v", api.Error)
	}
	if request.Filter != "active" || request.Name != "acme" {
		t.Fatalf("unexpected bind result: %+v", request)
	}
}

func TestBindUsesDeclaredFormBodyInsteadOfJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/records", strings.NewReader("name=form-value"))
	context.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var request queryAndBodyRequest
	api := Make(context).Bind(&request)
	if api.Error != nil {
		t.Fatalf("form bind failed: %v", api.Error)
	}
	if request.Name != "form-value" {
		t.Fatalf("form value was not bound: %+v", request)
	}
}

func TestBindReportsFinalValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/records/42", bytes.NewBufferString(`{}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Params = gin.Params{{Key: "id", Value: "42"}}

	var request mixedBindRequest
	api := Make(context).Bind(&request)
	if api.Error == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(api.Error.Error(), "Name") {
		t.Fatalf("validation error does not identify Name: %v", api.Error)
	}
}

func TestBindingConstructorIsDeterministicAndDefensive(t *testing.T) {
	constructor := &bindConstructor{}
	first := constructor.GetBindingForGin(&mixedBindRequest{})
	second := constructor.GetBindingForGin(&mixedBindRequest{})
	want := []binding.Binding{nil, binding.JSON}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("binding order = %v, want %v", bindingNames(first), bindingNames(want))
	}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("cached binding order = %v, want %v", bindingNames(second), bindingNames(want))
	}
	first[0] = binding.Query
	third := constructor.GetBindingForGin(&mixedBindRequest{})
	if !reflect.DeepEqual(third, want) {
		t.Fatalf("caller mutated cached binding plan: %v", bindingNames(third))
	}
	if bindings := constructor.GetBindingForGin(nil); len(bindings) != 0 {
		t.Fatalf("nil input produced bindings: %v", bindingNames(bindings))
	}
}

func bindingNames(bindings []binding.Binding) []string {
	names := make([]string, 0, len(bindings))
	for _, requestBinding := range bindings {
		names = append(names, bindingName(requestBinding))
	}
	return names
}
