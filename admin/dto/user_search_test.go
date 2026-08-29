package dto

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/search/gorms"
)

func TestUserSearchStatusAcceptsOnlySupportedExactValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, status := range []enum.Status{enum.Enabled, enum.Disabled, enum.Locked} {
		t.Run(status.String(), func(t *testing.T) {
			search, condition, err := bindUserSearchStatus(status.String())
			if err != nil {
				t.Fatalf("bind status %q: %v", status, err)
			}
			if search.Status != status {
				t.Fatalf("bound status = %q, want %q", search.Status, status)
			}
			arguments, ok := condition.Where["`status` = ?"]
			if !ok {
				t.Fatalf("status predicate = %#v, want exact predicate", condition.Where)
			}
			if len(arguments) != 1 || arguments[0] != status {
				t.Fatalf("status arguments = %#v, want [%q]", arguments, status)
			}
		})
	}
}

func TestUserSearchStatusRejectsInvalidValuesBeforeBuildingQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, status := range []string{
		"all",
		"ENABLED",
		"enabled OR 1=1",
		"enabled,disabled",
	} {
		t.Run(status, func(t *testing.T) {
			_, condition, err := bindUserSearchStatus(status)
			if err == nil {
				t.Fatalf("invalid status %q was accepted", status)
			}
			if len(condition.Where) != 0 {
				t.Fatalf("invalid status %q produced predicates: %#v", status, condition.Where)
			}
		})
	}
}

func TestUserSearchEmptyStatusDoesNotBuildPredicate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	search, condition, err := bindUserSearchStatus("")
	if err != nil {
		t.Fatalf("bind empty status: %v", err)
	}
	if search.Status != enum.Unknown {
		t.Fatalf("empty status = %q, want unknown", search.Status)
	}
	if len(condition.Where) != 0 {
		t.Fatalf("empty status produced predicates: %#v", condition.Where)
	}
}

func bindUserSearchStatus(status string) (UserSearch, *gorms.GormCondition, error) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/admin/api/users?status="+url.QueryEscape(status),
		nil,
	)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	search := UserSearch{}
	condition := &gorms.GormCondition{}
	if err := context.ShouldBindQuery(&search); err != nil {
		return search, condition, err
	}
	gorms.ResolveSearchQuery(gorms.Mysql, search, condition)
	return search, condition, nil
}
