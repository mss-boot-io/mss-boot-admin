package security

import (
	"crypto/rand"
	"encoding/hex"

	"golang.org/x/crypto/scrypt"
)

const (
	symbol = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()-_=+,.?/:;{}[]`~"
	letter = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func generateRandString(length int, characters string) string {
	if length <= 0 {
		return ""
	}
	chars := []byte(characters)
	characterCount := len(chars)
	if characterCount < 2 || characterCount > 256 {
		panic("security: random character set length must be between 2 and 256")
	}
	maximumRandomByte := 255 - (256 % characterCount)
	result := make([]byte, length)
	randomBytes := make([]byte, length+(length/4)+1)
	written := 0
	for written < length {
		if _, err := rand.Read(randomBytes); err != nil {
			panic("security: read random bytes: " + err.Error())
		}
		for _, randomByte := range randomBytes {
			value := int(randomByte)
			if value > maximumRandomByte {
				continue
			}
			result[written] = chars[value%characterCount]
			written++
			if written == length {
				break
			}
		}
	}
	return string(result)
}

func GenerateRandomKey20() string {
	return generateRandString(20, symbol)
}

func GenerateRandomKey16() string {
	return generateRandString(16, symbol)
}

func GenerateRandomKey6() string {
	return generateRandString(6, letter)
}

func SetPassword(password string, salt string) (string, error) {
	derived, err := scrypt.Key([]byte(password), []byte(salt), 16384, 8, 1, 32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(derived), nil
}
