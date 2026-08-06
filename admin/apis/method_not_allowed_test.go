package apis

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMethodNotAllowedIsSideEffectFreeTerminalHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/legacy-mutation", nil)

	methodNotAllowed(ctx)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != http.StatusMethodNotAllowed {
		t.Fatalf("business code = %d, want %d", body.Code, http.StatusMethodNotAllowed)
	}
	if !ctx.IsAborted() {
		t.Fatal("legacy mutation handler must abort the request chain")
	}
}
