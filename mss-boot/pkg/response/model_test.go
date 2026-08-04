package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
)

func TestResponseMutationAndClone(t *testing.T) {
	original := &response{}
	original.SetList(map[string]string{"name": "admin"})
	original.SetTraceID("trace-1")
	original.SetMsg("first", "second")
	original.SetCode(http.StatusTeapot)
	original.SetErrorCode("E_TEAPOT")
	original.SetStatus("error")

	clone, ok := original.Clone().(*response)
	if !ok || clone == original {
		t.Fatalf("clone = %#v", clone)
	}
	if clone.TraceID != "trace-1" || clone.ErrorMessage != "first,second" || clone.Code != http.StatusTeapot || clone.ErrorCode != "E_TEAPOT" || clone.Status != "error" {
		t.Fatalf("clone fields = %#v", clone)
	}
	clone.SetMsg("third")
	if original.ErrorMessage != "first,second" {
		t.Fatal("mutating clone changed original")
	}
}

func TestResponseErrorUsesTypedAndPlainErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := Default
	Default = &response{Response: Response{Host: "api.example"}}
	t.Cleanup(func() { Default = previous })

	for _, test := range []struct {
		name          string
		err           error
		message       []string
		wantErrorCode string
		wantText      string
	}{
		{
			name:          "typed error",
			err:           NewError("INVALID", "invalid input"),
			message:       []string{"request rejected"},
			wantErrorCode: "INVALID",
			wantText:      "request rejected,invalid input",
		},
		{
			name:     "plain error",
			err:      errors.New("database unavailable"),
			wantText: "database unavailable",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			context.Request.Header.Set(pkg.TrafficKey, "trace-request")

			(&response{}).Error(context, http.StatusUnprocessableEntity, test.err, test.message...)
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d", recorder.Code)
			}
			var body Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != http.StatusUnprocessableEntity || body.Status != "error" || body.TraceID != "trace-request" || body.Host != "api.example" {
				t.Fatalf("response body = %#v", body)
			}
			if body.ErrorCode != test.wantErrorCode || body.ErrorMessage != test.wantText {
				t.Fatalf("error body = %#v", body)
			}
			if value, exists := context.Get("status"); !exists || value != http.StatusUnprocessableEntity {
				t.Fatalf("context status = %#v, exists=%t", value, exists)
			}
		})
	}
}

func TestResponseOKStatusByMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		method     string
		data       any
		wantStatus int
		wantBody   string
	}{
		{method: http.MethodGet, data: map[string]string{"name": "admin"}, wantStatus: http.StatusOK, wantBody: `"name":"admin"`},
		{method: http.MethodPost, data: []string{"created"}, wantStatus: http.StatusCreated, wantBody: `"created"`},
		{method: http.MethodDelete, data: nil, wantStatus: http.StatusNoContent, wantBody: ""},
	} {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(test.method, "/", nil)
		(&response{}).OK(context, test.data)
		if recorder.Code != test.wantStatus {
			t.Fatalf("%s status = %d, want %d", test.method, recorder.Code, test.wantStatus)
		}
		if !strings.Contains(recorder.Body.String(), test.wantBody) {
			t.Fatalf("%s body = %q, want substring %q", test.method, recorder.Body.String(), test.wantBody)
		}
	}
}

func TestResponsePageOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/?current=2", nil)
	context.Request.Header.Set(pkg.TrafficKey, "page-trace")

	(&response{}).PageOK(context, []string{"a", "b"}, 7, 2, 2)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body struct {
		Page
		Data    []string `json:"data"`
		TraceID string   `json:"traceId"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode page response: %v", err)
	}
	if body.Count != 7 || body.Current != 2 || body.PageSize != 2 || body.TraceID != "page-trace" || len(body.Data) != 2 {
		t.Fatalf("page response = %#v", body)
	}
}

func TestResponseErrorsAndContextGuard(t *testing.T) {
	err := NewError("CODE", "message")
	if err.ErrorCode() != "CODE" || err.ErrorMsg() != "message" || !strings.Contains(err.Error(), "CODE") {
		t.Fatalf("default error = %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("nil context did not panic")
		}
	}()
	checkContext(nil)
}
