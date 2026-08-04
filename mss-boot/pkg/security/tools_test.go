package security

import (
	"strings"
	"testing"
)

func TestRandomKeyGenerators(t *testing.T) {
	for _, test := range []struct {
		name       string
		value      string
		length     int
		characters string
	}{
		{name: "20 character key", value: GenerateRandomKey20(), length: 20, characters: symbol},
		{name: "16 character key", value: GenerateRandomKey16(), length: 16, characters: symbol},
		{name: "6 character code", value: GenerateRandomKey6(), length: 6, characters: letter},
	} {
		t.Run(test.name, func(t *testing.T) {
			if len(test.value) != test.length {
				t.Fatalf("length = %d, want %d", len(test.value), test.length)
			}
			for _, character := range test.value {
				if !strings.ContainsRune(test.characters, character) {
					t.Fatalf("unexpected character %q in %q", character, test.value)
				}
			}
		})
	}
	if got := generateRandString(0, symbol); got != "" {
		t.Fatalf("zero-length random string = %q", got)
	}
	if got := generateRandString(-1, symbol); got != "" {
		t.Fatalf("negative-length random string = %q", got)
	}
}

func TestGenerateRandomStringRejectsInvalidCharacterSets(t *testing.T) {
	for _, characters := range []string{"", "x", strings.Repeat("x", 257)} {
		t.Run(strings.TrimSpace(characters), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("character set length %d did not panic", len(characters))
				}
			}()
			_ = generateRandString(1, characters)
		})
	}
}

func TestSetPasswordIsDeterministicAndSaltSensitive(t *testing.T) {
	first, err := SetPassword("password", "salt-a")
	if err != nil {
		t.Fatalf("derive password: %v", err)
	}
	second, err := SetPassword("password", "salt-a")
	if err != nil {
		t.Fatalf("derive password again: %v", err)
	}
	otherSalt, err := SetPassword("password", "salt-b")
	if err != nil {
		t.Fatalf("derive password with another salt: %v", err)
	}
	if first == "" || len(first) != 64 || first != second {
		t.Fatalf("deterministic password values first=%q second=%q", first, second)
	}
	if first == otherSalt {
		t.Fatal("different salts produced the same password verifier")
	}
}
