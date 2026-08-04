package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kamva/mgm/v3"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions/k8s"
)

type controllerTestModel struct {
	mgm.DefaultModel `bson:",inline"`
}

func (*controllerTestModel) TableName() string {
	return "Controller_Test_Records"
}

type controllerTestAction struct {
	name    string
	handler gin.HandlerFunc
}

func (e controllerTestAction) String() string {
	return e.name
}

func (e controllerTestAction) Handler() gin.HandlersChain {
	return gin.HandlersChain{e.handler}
}

func TestMongoActionFailsClosedWhenAuthHandlerIsMissing(t *testing.T) {
	previous := response.AuthHandler
	response.AuthHandler = nil
	defer func() { response.AuthHandler = previous }()

	controller := NewSimple(
		WithModel(&controllerTestModel{}),
		WithModelProvider(actions.ModelProviderMgm),
		WithAuth(true),
	)
	action := controller.GetAction(response.Get)
	if action == nil {
		t.Fatal("expected Mongo get action")
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/records/:id", action.Handler()...)
	request := httptest.NewRequest(http.MethodGet, "/records/not-an-object-id", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected fail-closed 500 response, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAuthenticationRunsBeforeProviderAction(t *testing.T) {
	previous := response.AuthHandler
	response.AuthHandler = func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	}
	defer func() { response.AuthHandler = previous }()

	controller := NewSimple(
		WithModel(&controllerTestModel{}),
		WithModelProvider(actions.ModelProviderMgm),
		WithAuth(true),
	)
	action := controller.GetAction(response.Get)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/records/:id", action.Handler()...)
	request := httptest.NewRequest(http.MethodGet, "/records/not-an-object-id", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected auth middleware to stop the action, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestPathPreservesLegacyModelNamingAndSupportsKubernetes(t *testing.T) {
	model := &controllerTestModel{}
	gormController := NewSimple(
		WithModel(model),
		WithModelProvider(actions.ModelProviderGorm),
	)
	wantLegacyPath := normalizePath(mgm.CollName(model))
	if got := gormController.Path(); got != wantLegacyPath {
		t.Fatalf("GORM path = %q, want legacy path %q", got, wantLegacyPath)
	}

	kubernetesController := NewSimple(
		WithModelProvider(actions.ModelProviderK8S),
		WithResourceType(k8s.CronJob),
	)
	if got := kubernetesController.Path(); got != "cronjob" {
		t.Fatalf("Kubernetes path = %q", got)
	}
}

func TestCommonMiddlewareRunsOnce(t *testing.T) {
	calls := 0
	controller := NewSimple(
		WithHandlers(gin.HandlersChain{func(c *gin.Context) {
			calls++
			c.Next()
		}}),
		WithAction(controllerTestAction{
			name: response.Get,
			handler: func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			},
		}),
	)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/records", controller.Handlers()...)
	group.GET("", controller.GetAction(response.Get).Handler()...)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/records", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if calls != 1 {
		t.Fatalf("common middleware ran %d times, want 1", calls)
	}
}

func TestNoAuthActionBypassesMissingGlobalAuthHandler(t *testing.T) {
	previous := response.AuthHandler
	response.AuthHandler = nil
	defer func() { response.AuthHandler = previous }()

	controller := NewSimple(
		WithAuth(true),
		WithNoAuthAction(response.Get),
		WithAction(controllerTestAction{
			name: response.Get,
			handler: func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			},
		}),
	)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/records", controller.GetAction(response.Get).Handler()...)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/records", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected unauthenticated action to run, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
