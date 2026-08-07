package config

import (
	"strings"
	"testing"
)

func TestValidateProductionAuthKeyFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		mode    Mode
		key     string
		wantErr bool
	}{
		{name: "development default remains usable", mode: ModeDev, key: insecureDefaultAuthKey},
		{name: "test empty remains usable", mode: ModeTest, key: ""},
		{name: "production empty", mode: ModeProd, key: "", wantErr: true},
		{name: "production whitespace", mode: ModeProd, key: "   ", wantErr: true},
		{name: "production default", mode: ModeProd, key: insecureDefaultAuthKey, wantErr: true},
		{name: "production short", mode: ModeProd, key: "short-but-custom", wantErr: true},
		{name: "production strong override", mode: ModeProd, key: "0123456789abcdef0123456789abcdef"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateProductionAuthKey(test.mode, test.key)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateProductionAuthKey(%q) error = %v, wantErr %v", test.key, err, test.wantErr)
			}
			if err != nil && (contains(err.Error(), test.key) || contains(err.Error(), insecureDefaultAuthKey)) {
				t.Fatalf("validation error leaked auth key material: %v", err)
			}
		})
	}
}

func contains(value, secret string) bool {
	return secret != "" && len(secret) > 4 && strings.Contains(value, secret)
}
