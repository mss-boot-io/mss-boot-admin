package models

import (
	"errors"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const EmailIdentityUniqueIndex = "ux_mss_boot_users_active_canonical_email"

var (
	ErrEmailIdentityInvalid   = errors.New("email identity is invalid")
	ErrEmailIdentityExists    = errors.New("email identity already exists")
	ErrEmailIdentityAmbiguous = errors.New("email identity is ambiguous")
)

// CanonicalEmailIdentity is the single model-layer entry point for an email
// identity. Email remains optional on User; callers invoke this function only
// for a non-empty identity they intend to persist or query.
func CanonicalEmailIdentity(value string) (string, error) {
	canonical, ok := pkg.CanonicalEmail(value)
	if !ok {
		return "", ErrEmailIdentityInvalid
	}
	return canonical, nil
}

// CanonicalizeOptionalEmail preserves the legacy optional-email contract while
// ensuring every non-empty value uses the authoritative identity form.
func CanonicalizeOptionalEmail(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return CanonicalEmailIdentity(value)
}

// NormalizeEmailIdentityPersistenceError maps only the named canonical-email
// constraint to ErrEmailIdentityExists. It deliberately does not classify a
// generic duplicate-key error: mss_boot_users has other unique identities and
// callers must never turn an unrelated conflict into an email conflict.
func NormalizeEmailIdentityPersistenceError(err error) error {
	if err == nil || errors.Is(err, ErrEmailIdentityInvalid) ||
		errors.Is(err, ErrEmailIdentityExists) ||
		errors.Is(err, ErrEmailIdentityAmbiguous) {
		return err
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, strings.ToLower(EmailIdentityUniqueIndex)) &&
		(strings.Contains(message, "unique") ||
			strings.Contains(message, "duplicate") ||
			strings.Contains(message, "constraint")) {
		return ErrEmailIdentityExists
	}
	return err
}

// NormalizeEmailIdentityCreateError first confirms the authoritative active
// identity in the database. This keeps conflict classification stable when
// GORM TranslateError has erased a driver constraint name. The lookup is
// silent so an email address cannot be copied into SQL logs.
func NormalizeEmailIdentityCreateError(db *gorm.DB, email string, createErr error) error {
	if createErr == nil {
		return nil
	}
	canonical, canonicalErr := CanonicalizeOptionalEmail(email)
	if canonicalErr != nil {
		return canonicalErr
	}
	if db != nil && canonical != "" {
		var count int64
		lookup := db.Session(&gorm.Session{Logger: logger.Discard}).
			Model(&User{}).
			Where("LOWER(TRIM(email)) = ?", canonical).
			Limit(1).
			Count(&count)
		if lookup.Error == nil && count != 0 {
			return ErrEmailIdentityExists
		}
	}
	return NormalizeEmailIdentityPersistenceError(createErr)
}

func createUserWithCanonicalEmail(db *gorm.DB, user *User) error {
	if db == nil {
		return errors.New("user database is unavailable")
	}
	if user == nil {
		return errors.New("user is nil")
	}
	quiet := db.Session(&gorm.Session{Logger: logger.Discard})
	canonical, err := CanonicalizeOptionalEmail(user.Email)
	if err != nil {
		return err
	}
	user.Email = canonical
	if canonical != "" {
		var existing int64
		result := quiet.Model(&User{}).
			Where("LOWER(TRIM(email)) = ?", canonical).
			Limit(1).
			Count(&existing)
		if result.Error != nil {
			return errors.New("email identity lookup failed")
		}
		if existing != 0 {
			return ErrEmailIdentityExists
		}
	}
	err = quiet.Create(user).Error
	return NormalizeEmailIdentityCreateError(quiet, user.Email, err)
}

func createOAuthIdentityWithoutEmailMerge(db *gorm.DB, identity *UserOAuth2) error {
	if db == nil {
		return errors.New("oauth identity database is unavailable")
	}
	if identity == nil {
		return errors.New("oauth identity is nil")
	}
	quiet := db.Session(&gorm.Session{Logger: logger.Discard})
	user := identity.User
	if user == nil {
		err := quiet.Create(identity).Error
		return NormalizeEmailIdentityCreateError(quiet, identity.Email, err)
	}

	// GORM's automatic association upsert uses ON CONFLICT DO NOTHING. For a
	// canonical-email collision that can otherwise create an OAuth row pointing
	// at a newly generated but nonexistent user ID. Create both rows explicitly
	// in one transaction and never look up or attach an account by email.
	identity.User = nil
	err := quiet.Transaction(func(tx *gorm.DB) error {
		if err := createUserWithCanonicalEmail(tx, user); err != nil {
			return err
		}
		identity.UserID = user.ID
		return tx.Omit("User").Create(identity).Error
	})
	identity.User = user
	err = NormalizeEmailIdentityCreateError(quiet, user.Email, err)
	if err == nil {
		recordUserCreatedFromDB(quiet, user)
	}
	return err
}
