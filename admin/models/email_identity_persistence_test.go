package models

import (
	"errors"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"gorm.io/gorm"
)

func TestCanonicalEmailIdentityReturnsTypedValidationAndCanonicalValue(t *testing.T) {
	canonical, err := CanonicalEmailIdentity(" Person@EXAMPLE.COM ")
	if err != nil || canonical != "person@example.com" {
		t.Fatalf("canonical identity = %q, %v", canonical, err)
	}
	if _, err := CanonicalEmailIdentity("not-an-address"); !errors.Is(err, ErrEmailIdentityInvalid) {
		t.Fatalf("invalid identity error = %v, want ErrEmailIdentityInvalid", err)
	}
	if optional, err := CanonicalizeOptionalEmail(""); err != nil || optional != "" {
		t.Fatalf("optional empty identity = %q, %v", optional, err)
	}
}

func TestNormalizeEmailIdentityPersistenceErrorIsConstraintSpecific(t *testing.T) {
	for _, conflict := range []error{
		errors.New("UNIQUE constraint failed: index 'ux_mss_boot_users_active_canonical_email'"),
		errors.New("Error 1062: Duplicate entry for key 'ux_mss_boot_users_active_canonical_email'"),
		errors.New("SQLSTATE 23505: violates unique constraint ux_mss_boot_users_active_canonical_email"),
	} {
		if err := NormalizeEmailIdentityPersistenceError(conflict); !errors.Is(err, ErrEmailIdentityExists) {
			t.Fatalf("constraint conflict %q normalized to %v", conflict, err)
		}
	}
	unrelated := errors.New("UNIQUE constraint failed: mss_boot_users.username")
	if got := NormalizeEmailIdentityPersistenceError(unrelated); got != unrelated {
		t.Fatalf("unrelated unique error normalized to %v", got)
	}
}

func TestCanonicalEmailCreateNormalizesConflictWithoutLeakingOtherUniqueErrors(t *testing.T) {
	db := setupGithubVerifyTest(t, githubTestAppConfig{})
	installCanonicalEmailIdentitySQLiteIndex(t, db)
	role := defaultEmailIdentityTestRole(t, db)

	owner := &User{UserLogin: UserLogin{
		RoleID:   role.ID,
		Username: "canonical-owner",
		Email:    " Person@EXAMPLE.COM ",
		Password: "owner-password",
		Status:   enum.Enabled,
	}}
	if err := createUserWithCanonicalEmail(db, owner); err != nil {
		t.Fatalf("create canonical owner: %v", err)
	}
	if owner.Email != "person@example.com" {
		t.Fatalf("persisted canonical email = %q", owner.Email)
	}

	contender := &User{UserLogin: UserLogin{
		RoleID:   role.ID,
		Username: "canonical-contender",
		Email:    "PERSON@example.com",
		Password: "contender-password",
		Status:   enum.Enabled,
	}}
	if err := createUserWithCanonicalEmail(db, contender); !errors.Is(err, ErrEmailIdentityExists) {
		t.Fatalf("duplicate canonical create error = %v, want ErrEmailIdentityExists", err)
	}
	var count int64
	if err := db.Model(&User{}).Where("email = ?", "person@example.com").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("active canonical owners = %d, want 1", count)
	}
	rawErr := db.Exec(
		"INSERT INTO mss_boot_users (id, email) VALUES (?, ?)",
		"raw-constraint-contender",
		" PERSON@EXAMPLE.COM ",
	).Error
	if rawErr == nil || !errors.Is(
		NormalizeEmailIdentityPersistenceError(rawErr),
		ErrEmailIdentityExists,
	) {
		t.Fatalf("actual SQLite expression-index error normalized to %v", rawErr)
	}
}

func TestOAuthProvisioningNeverMergesByEmail(t *testing.T) {
	db := setupGithubVerifyTest(t, githubTestAppConfig{})
	installCanonicalEmailIdentitySQLiteIndex(t, db)
	role := defaultEmailIdentityTestRole(t, db)

	owner := &User{UserLogin: UserLogin{
		RoleID:   role.ID,
		Username: "local-email-owner",
		Email:    "owner@example.com",
		Password: "owner-password",
		Status:   enum.Enabled,
	}}
	if err := createUserWithCanonicalEmail(db, owner); err != nil {
		t.Fatal(err)
	}
	identity := &UserOAuth2{
		Provider: pkg.GithubLoginProvider,
		OpenID:   "new-provider-identity",
		Email:    "OWNER@EXAMPLE.COM",
		User: &User{UserLogin: UserLogin{
			RoleID:                role.ID,
			Username:              "oauth-provisioned-user",
			Email:                 "OWNER@EXAMPLE.COM",
			Password:              "generated-password",
			LocalPasswordDisabled: true,
			Status:                enum.Enabled,
		}},
	}
	err := createOAuthIdentityWithoutEmailMerge(db, identity)
	if !errors.Is(err, ErrEmailIdentityExists) {
		t.Fatalf("OAuth provisioning conflict = %v, want ErrEmailIdentityExists", err)
	}
	var users int64
	var identities int64
	if err := db.Model(&User{}).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&UserOAuth2{}).Count(&identities).Error; err != nil {
		t.Fatal(err)
	}
	if users != 1 || identities != 0 {
		t.Fatalf("OAuth email conflict users=%d identities=%d, want 1/0 without merge", users, identities)
	}
	var unchanged User
	if err := db.Where("id = ?", owner.ID).Take(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Username != owner.Username {
		t.Fatalf("OAuth conflict mutated owner username to %q", unchanged.Username)
	}
}

func TestUserBeforeSaveRejectsInvalidEmailWithTypedError(t *testing.T) {
	user := &User{UserLogin: UserLogin{Email: "invalid-address"}}
	if err := user.BeforeSave(nil); !errors.Is(err, ErrEmailIdentityInvalid) {
		t.Fatalf("BeforeSave invalid email error = %v", err)
	}
}

func TestInvalidEmailLoginRemainsAClientAuthenticationFailure(t *testing.T) {
	login := &UserLogin{
		Provider: pkg.EmailLoginProvider,
		Email:    "invalid-address",
		Captcha:  "123456",
	}
	ok, principal, err := login.Verify(newGithubVerifyContext("http://127.0.0.1"))
	if err != nil || ok || principal != nil {
		t.Fatalf("invalid email login = %v, %#v, %v; want false, nil, nil", ok, principal, err)
	}
}

func installCanonicalEmailIdentitySQLiteIndex(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(
		"CREATE UNIQUE INDEX " + EmailIdentityUniqueIndex +
			" ON mss_boot_users (LOWER(TRIM(email)))" +
			" WHERE deleted_at IS NULL AND TRIM(email) <> ''",
	).Error; err != nil {
		t.Fatal(err)
	}
}

func defaultEmailIdentityTestRole(t *testing.T, db *gorm.DB) Role {
	t.Helper()
	var role Role
	if err := db.Where("\"default\" = ?", true).Take(&role).Error; err != nil {
		t.Fatal(err)
	}
	return role
}
