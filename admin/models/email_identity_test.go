package models

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/gormdb"
	storagecache "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type acceptingEmailChallenge struct{}

func (acceptingEmailChallenge) Ready(context.Context) error { return nil }

func (acceptingEmailChallenge) Issue(
	context.Context,
	string,
	string,
	storagecache.ChallengePurpose,
	func(context.Context, string) error,
) error {
	return errors.New("unexpected Issue call")
}

func (acceptingEmailChallenge) VerifyChallenge(
	context.Context,
	string,
	storagecache.ChallengePurpose,
	string,
) (bool, error) {
	return true, nil
}

func TestEmailRegistrationRejectsExistingCanonicalIdentity(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{
		"security:registerEnabled": "true",
		"security:emailEnabled":    "true",
	})
	previousChallenge := center.GetChallenge()
	center.SetChallenge(acceptingEmailChallenge{})
	t.Cleanup(func() { center.SetChallenge(previousChallenge) })

	existing := &User{UserLogin: UserLogin{
		Email:    "Person@example.com",
		Username: "existing-email-owner",
		Password: "existing-password",
		Status:   enum.Enabled,
	}}
	if err := database.Create(existing).Error; err != nil {
		t.Fatalf("seed existing email identity: %v", err)
	}
	login := &UserLogin{
		Provider: pkg.EmailRegisterProvider,
		Email:    "Person@EXAMPLE.COM",
		Captcha:  "123456",
		Password: "replacement-password",
	}
	ok, principal, err := login.Verify(newGithubVerifyContext("http://127.0.0.1"))
	if err != nil || ok || principal != nil {
		t.Fatalf("duplicate registration = %v, %#v, %v; want false, nil, nil", ok, principal, err)
	}
	var count int64
	if err = database.Model(&User{}).Where("email = ?", "person@example.com").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("canonical identity count = %d, %v; want 1, nil", count, err)
	}
}

func TestEmailRegistrationUsesBoundedOpaqueUsername(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{
		"security:registerEnabled": "true",
		"security:emailEnabled":    "true",
	})
	previousChallenge := center.GetChallenge()
	center.SetChallenge(acceptingEmailChallenge{})
	t.Cleanup(func() { center.SetChallenge(previousChallenge) })

	const address = "very.long.email.identity+registration@example.test"
	login := &UserLogin{
		Provider: pkg.EmailRegisterProvider,
		Email:    address,
		Captcha:  "123456",
		Password: "replacement-password",
	}
	ok, principal, err := login.Verify(newGithubVerifyContext("http://127.0.0.1"))
	if err != nil || !ok || principal == nil {
		t.Fatalf("long email registration = %v, %#v, %v; want success", ok, principal, err)
	}
	registered := principal.(*User)
	if registered.Email != address || registered.Username == address || len(registered.Username) == 0 || len(registered.Username) > 20 {
		t.Fatalf("registered identity email=%q username=%q; want canonical email and bounded opaque username", registered.Email, registered.Username)
	}
	var persisted User
	if err = database.Where("id = ?", registered.ID).Take(&persisted).Error; err != nil {
		t.Fatalf("load long email registration: %v", err)
	}
	if persisted.Email != address || persisted.Username != registered.Username {
		t.Fatalf("persisted identity email=%q username=%q", persisted.Email, persisted.Username)
	}
}

func TestGetUserByEmailFailsClosedOnAmbiguousIdentity(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{})
	for index := range 2 {
		user := &User{UserLogin: UserLogin{
			Email:    "ambiguous@example.com",
			Username: fmt.Sprintf("ambiguous-%d", index),
			Password: "test-password",
			Status:   enum.Enabled,
		}}
		if err := database.Create(user).Error; err != nil {
			t.Fatalf("seed ambiguous identity %d: %v", index, err)
		}
	}
	if user, err := GetUserByEmail(newGithubVerifyContext("http://127.0.0.1"), "ambiguous@EXAMPLE.COM"); user != nil || !errors.Is(err, ErrEmailIdentityAmbiguous) {
		t.Fatalf("ambiguous lookup = %#v, %v; want nil, ErrEmailIdentityAmbiguous", user, err)
	}
}

func TestEmailIdentityOperationsDoNotEmitSensitiveSQL(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{
		"security:registerEnabled": "true",
		"security:emailEnabled":    "true",
	})
	previousChallenge := center.GetChallenge()
	center.SetChallenge(acceptingEmailChallenge{})
	t.Cleanup(func() { center.SetChallenge(previousChallenge) })
	existing := &User{UserLogin: UserLogin{
		Email:    "audit-existing@example.com",
		Username: "audit-existing",
		Password: "existing-password",
		Status:   enum.Enabled,
	}}
	if err := database.Create(existing).Error; err != nil {
		t.Fatalf("seed email audit identity: %v", err)
	}

	var output bytes.Buffer
	observed := database.Session(&gorm.Session{Logger: logger.New(
		log.New(&output, "", 0),
		logger.Config{LogLevel: logger.Info, Colorful: false},
	)})
	gormdb.DB = observed
	if _, err := GetUserByEmail(newGithubVerifyContext("http://127.0.0.1"), "audit-existing@example.com"); err != nil {
		t.Fatalf("lookup email audit identity: %v", err)
	}
	login := &UserLogin{
		Provider: pkg.EmailRegisterProvider,
		Email:    "audit-new-registration@example.com",
		Captcha:  "123456",
		Password: "registration-password-sentinel",
	}
	if ok, principal, err := login.Verify(newGithubVerifyContext("http://127.0.0.1")); err != nil || !ok || principal == nil {
		t.Fatalf("register email audit identity = %v, %#v, %v", ok, principal, err)
	}
	for _, sensitive := range []string{
		"audit-existing@example.com",
		"audit-new-registration@example.com",
		"registration-password-sentinel",
	} {
		if strings.Contains(output.String(), sensitive) {
			t.Fatalf("identity SQL log leaked %q: %s", sensitive, output.String())
		}
	}
}

func TestConcurrentEmailRegistrationFailsClosedWithoutAmbiguousIdentity(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{
		"security:registerEnabled": "true",
		"security:emailEnabled":    "true",
	})
	previousChallenge := center.GetChallenge()
	center.SetChallenge(acceptingEmailChallenge{})
	t.Cleanup(func() { center.SetChallenge(previousChallenge) })

	const contenders = 20
	start := make(chan struct{})
	var wait sync.WaitGroup
	var successes atomic.Int64
	for index := range contenders {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			login := &UserLogin{
				Provider: pkg.EmailRegisterProvider,
				Email:    "concurrent@EXAMPLE.COM",
				Captcha:  "123456",
				Password: fmt.Sprintf("password-%02d", index),
			}
			ok, principal, _ := login.Verify(newGithubVerifyContext("http://127.0.0.1"))
			if ok && principal != nil {
				successes.Add(1)
			}
		}(index)
	}
	close(start)
	wait.Wait()
	var count int64
	if err := database.Model(&User{}).Where("email = ?", "concurrent@example.com").Count(&count).Error; err != nil {
		t.Fatalf("count concurrent identities: %v", err)
	}
	if count > 1 || successes.Load() > 1 {
		t.Fatalf("concurrent registration count=%d successes=%d; want fail-closed outcomes with at most one", count, successes.Load())
	}
}
