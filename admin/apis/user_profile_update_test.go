package apis

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
)

func TestNormalizeSelfProfileUpdatesUsesAnExactClearableAllowlist(t *testing.T) {
	updates, err := normalizeSelfProfileUpdates(map[string]any{
		"name":    "",
		"profile": "",
		"tags":    []any{},
	})
	if err != nil {
		t.Fatalf("normalize profile update: %v", err)
	}
	want := map[string]any{
		"name":    "",
		"profile": "",
		"tags":    models.ArrayString{},
	}
	if !reflect.DeepEqual(updates, want) {
		t.Fatalf("updates = %#v, want %#v", updates, want)
	}

	for _, input := range []map[string]any{
		{"email": "new@example.test"},
		{"username": "renamed"},
		{"unknown": "value"},
		{"phone": 123},
		{"tags": []any{"valid", 42}},
	} {
		if _, err := normalizeSelfProfileUpdates(input); err == nil {
			t.Fatalf("input %#v was accepted", input)
		}
	}
}

func TestUpdateUserInfoPersistsExplicitEmptyProfileValues(t *testing.T) {
	db := prepareInteractiveSecurityTestDB(t)
	for _, statement := range []string{
		"ALTER TABLE mss_boot_users ADD COLUMN name TEXT",
		"ALTER TABLE mss_boot_users ADD COLUMN profile TEXT",
		"ALTER TABLE mss_boot_users ADD COLUMN tags TEXT",
		"UPDATE mss_boot_users SET name = 'Owner', profile = 'About', tags = 'one|two' WHERE id = 'user-1'",
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare profile columns: %v", err)
		}
	}

	principal := &models.User{}
	principal.ID = "user-1"
	recorder := executeHandlerWithIdentity(
		t,
		principal,
		http.MethodPut,
		"/admin/api/user/userInfo",
		"/admin/api/user/userInfo",
		(&User{}).UpdateUserInfo,
		`{"name":"","profile":"","tags":[]}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s, want 200", recorder.Code, recorder.Body.String())
	}

	var stored struct {
		Name    string
		Profile string
		Tags    string
	}
	if err := db.Table("mss_boot_users").Select("name", "profile", "tags").Where("id = ?", "user-1").Take(&stored).Error; err != nil {
		t.Fatalf("load updated profile: %v", err)
	}
	if stored.Name != "" || stored.Profile != "" || stored.Tags != "" {
		t.Fatalf("stored profile = %#v, want explicit empty values", stored)
	}
}
