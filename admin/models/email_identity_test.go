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
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	runtimechallenge "github.com/mss-boot-io/mss-boot-admin/mss-boot/runtime/challenge"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type acceptingEmailChallenge struct{}

func (acceptingEmailChallenge) Ready(context.Context) error { return nil }

func (acceptingEmailChallenge) BeginIssue(
	context.Context,
	runtimechallenge.BeginRequest,
) (runtimechallenge.BeginOutcome, error) {
	return runtimechallenge.BeginOutcome{}, errors.New("unexpected BeginIssue call")
}

func (acceptingEmailChallenge) Commit(context.Context, *runtimechallenge.Reservation) error {
	return errors.New("unexpected Commit call")
}

func (acceptingEmailChallenge) Abort(context.Context, *runtimechallenge.Reservation) error {
	return errors.New("unexpected Abort call")
}

func (acceptingEmailChallenge) Verify(
	context.Context,
	runtimechallenge.VerifyRequest,
) (runtimechallenge.VerifyOutcome, error) {
	return runtimechallenge.VerifyVerified, nil
}

func TestEmailRegistrationRejectsExistingCanonicalIdentity(t *testing.T) {
	database := setupGithubVerifyTest(t, githubTestAppConfig{
		"security:registerEnabled": "true",
		"security:emailEnabled":    "true",
	})
	previousChallenge := center.GetRuntimeChallenge()
	center.SetRuntimeChallenge(acceptingEmailChallenge{})
	t.Cleanup(func() { center.SetRuntimeChallenge(previousChallenge) })

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
	previousChallenge := center.GetRuntimeChallenge()
	center.SetRuntimeChallenge(acceptingEmailChallenge{})
	t.Cleanup(func() { center.SetRuntimeChallenge(previousChallenge) })

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
	previousChallenge := center.GetRuntimeChallenge()
	center.SetRuntimeChallenge(acceptingEmailChallenge{})
	t.Cleanup(func() { center.SetRuntimeChallenge(previousChallenge) })
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
	installCanonicalEmailIdentitySQLiteIndex(t, database)
	previousChallenge := center.GetRuntimeChallenge()
	center.SetRuntimeChallenge(acceptingEmailChallenge{})
	t.Cleanup(func() { center.SetRuntimeChallenge(previousChallenge) })

	const contenders = 20
	start := make(chan struct{})
	var wait sync.WaitGroup
	var successes atomic.Int64
	var invalidOutcomes atomic.Int64
	winnerIDs := make(chan string, contenders)
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
			ok, principal, err := login.Verify(newGithubVerifyContext("http://127.0.0.1"))
			if !ok {
				if principal != nil {
					invalidOutcomes.Add(1)
				}
				return
			}
			user, isUser := principal.(*User)
			if err != nil || !isUser || user.ID == "" {
				invalidOutcomes.Add(1)
				return
			}
			successes.Add(1)
			winnerIDs <- user.ID
		}(index)
	}
	close(start)
	wait.Wait()
	close(winnerIDs)
	if invalidOutcomes.Load() != 0 {
		t.Fatalf("concurrent registration returned %d inconsistent outcomes", invalidOutcomes.Load())
	}
	if successes.Load() != 1 || len(winnerIDs) != 1 {
		t.Fatalf("concurrent registration successes=%d principals=%d, want exactly one", successes.Load(), len(winnerIDs))
	}
	winnerID := <-winnerIDs
	var owner User
	if err := database.Where("email = ?", "concurrent@example.com").Take(&owner).Error; err != nil {
		t.Fatalf("load canonical owner: %v", err)
	}
	if owner.ID != winnerID {
		t.Fatalf("persisted owner ID %q does not match successful principal %q", owner.ID, winnerID)
	}
	var count int64
	if err := database.Model(&User{}).Count(&count).Error; err != nil {
		t.Fatalf("count concurrent identities: %v", err)
	}
	if count != 1 {
		t.Fatalf("concurrent registration persisted %d users, want exactly one without loser residue", count)
	}
}
