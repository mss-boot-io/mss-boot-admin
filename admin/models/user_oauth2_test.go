package models

import (
	"path/filepath"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserOAuthIdentityKeyCanonicalContract(t *testing.T) {
	tests := []struct {
		name     string
		provider pkg.LoginProvider
		openID   string
		unionID  string
		want     string
		wantErr  bool
	}{
		{name: "github", provider: " GitHub ", openID: " 42 ", want: "github:42"},
		{name: "lark", provider: " LARK ", openID: "ignored", unionID: " on_abc ", want: "lark:on_abc"},
		{name: "github requires open id", provider: pkg.GithubLoginProvider, unionID: "wrong-field", wantErr: true},
		{name: "lark requires union id", provider: pkg.LarkLoginProvider, openID: "wrong-field", wantErr: true},
		{name: "unsupported provider", provider: pkg.EmailLoginProvider, openID: "42", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UserOAuthIdentityKey(tt.provider, tt.openID, tt.unionID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("UserOAuthIdentityKey() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("UserOAuthIdentityKey() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("UserOAuthIdentityKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUserOAuth2CreatePopulatesIdentityKeyAndFailsClosed(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "oauth-identity-model.db")
	db, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&UserOAuth2{}); err != nil {
		t.Fatalf("migrate OAuth identity model: %v", err)
	}

	githubIdentity := &UserOAuth2{
		UserID:   "github-user",
		Provider: " GitHub ",
		OpenID:   " 42 ",
	}
	if err := db.Create(githubIdentity).Error; err != nil {
		t.Fatalf("create GitHub identity: %v", err)
	}
	if githubIdentity.IdentityKey == nil || *githubIdentity.IdentityKey != "github:42" {
		t.Fatalf("GitHub identity key = %#v, want github:42", githubIdentity.IdentityKey)
	}
	if githubIdentity.Provider != pkg.GithubLoginProvider || githubIdentity.OpenID != "42" {
		t.Fatalf("GitHub identity was not normalized: %#v", githubIdentity)
	}

	larkIdentity := &UserOAuth2{
		UserID:   "lark-user",
		Provider: pkg.LarkLoginProvider,
		UnionID:  " on_abc ",
	}
	if err := db.Create(larkIdentity).Error; err != nil {
		t.Fatalf("create Lark identity: %v", err)
	}
	if larkIdentity.IdentityKey == nil || *larkIdentity.IdentityKey != "lark:on_abc" {
		t.Fatalf("Lark identity key = %#v, want lark:on_abc", larkIdentity.IdentityKey)
	}

	invalid := []*UserOAuth2{
		{UserID: "missing-github", Provider: pkg.GithubLoginProvider},
		{UserID: "missing-lark", Provider: pkg.LarkLoginProvider},
		{UserID: "unsupported", Provider: pkg.EmailLoginProvider, OpenID: "42"},
	}
	for _, identity := range invalid {
		if err := db.Create(identity).Error; err == nil {
			t.Fatalf("invalid identity %#v was persisted", identity)
		}
	}

	duplicate := &UserOAuth2{
		UserID:   "different-user",
		Provider: pkg.GithubLoginProvider,
		OpenID:   "42",
	}
	if err := db.Create(duplicate).Error; err == nil {
		t.Fatal("duplicate provider identity was persisted")
	}

	githubIdentity.OpenID = "84"
	if err := db.Save(githubIdentity).Error; err != nil {
		t.Fatalf("update GitHub identity: %v", err)
	}
	if githubIdentity.IdentityKey == nil || *githubIdentity.IdentityKey != "github:84" {
		t.Fatalf("updated identity key = %#v, want github:84", githubIdentity.IdentityKey)
	}
}
