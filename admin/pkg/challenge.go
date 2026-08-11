package pkg

import (
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage/cache"
)

const (
	EmailLoginChallengePurpose    cache.ChallengePurpose = "email-login"
	EmailRegisterChallengePurpose cache.ChallengePurpose = "email-registration"
	PasswordResetChallengePurpose cache.ChallengePurpose = "password-reset"
)

// CanonicalEmail defines the identity representation shared by challenge
// issuance, verification, and authoritative account lookup. MSS treats email
// addresses as case-insensitive login identifiers so Redis, PostgreSQL,
// MySQL, and SQLite cannot disagree through provider-specific collations.
func CanonicalEmail(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 100 || strings.Count(value, "@") != 1 {
		return "", false
	}
	separator := strings.LastIndexByte(value, '@')
	if separator <= 0 || separator == len(value)-1 {
		return "", false
	}
	local := value[:separator]
	domain := value[separator+1:]
	if !validEmailLocalPart(local) || !validEmailDomain(domain) {
		return "", false
	}
	return strings.ToLower(local + "@" + domain), true
}

func validEmailLocalPart(value string) bool {
	if value == "" || len(value) > 64 || value[0] == '.' || value[len(value)-1] == '.' || strings.Contains(value, "..") {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			strings.ContainsRune(".!#$%&'*+-/=?^_`{|}~", rune(character)) {
			continue
		}
		return false
	}
	return true
}

func validEmailDomain(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for index := range len(label) {
			character := label[index]
			if character >= 'a' && character <= 'z' ||
				character >= 'A' && character <= 'Z' ||
				character >= '0' && character <= '9' || character == '-' {
				continue
			}
			return false
		}
	}
	return true
}
