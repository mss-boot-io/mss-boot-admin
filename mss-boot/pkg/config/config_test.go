package config

import (
	"os"
	"testing"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/19 11:24:58
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/19 11:24:58
 */

func Test_parseTemplateWithEnv(t *testing.T) {
	type args struct {
		rb []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test0",
			args: args{
				rb: []byte(`{{.Env.TEST}}`),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTemplateWithEnv(tt.args.rb)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTemplateWithEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_ = got
		})
	}
}

func Test_parseTemplateWithEnvFallback(t *testing.T) {
	t.Setenv("MSS_BOOT_EXACT", "exact-value")
	t.Setenv("MSS_BOOT_UPPER", "upper-value")
	t.Setenv("mss_boot_lower", "lower-value")

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "missing env renders empty string",
			template: `value: "{{.Env.MSS_BOOT_MISSING}}"`,
			want:     `value: ""`,
		},
		{
			name:     "exact env key wins",
			template: `value: "{{.Env.MSS_BOOT_EXACT}}"`,
			want:     `value: "exact-value"`,
		},
		{
			name:     "uppercase fallback is supported",
			template: `value: "{{.Env.mss_boot_upper}}"`,
			want:     `value: "upper-value"`,
		},
		{
			name:     "lowercase fallback is supported",
			template: `value: "{{.Env.MSS_BOOT_LOWER}}"`,
			want:     `value: "lower-value"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTemplateWithEnv([]byte(tt.template))
			if err != nil {
				t.Fatalf("parseTemplateWithEnv() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("parseTemplateWithEnv() = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func Test_getValueFromEnvFallbackMap(t *testing.T) {
	t.Setenv("MSS_BOOT_DIRECT", "direct-value")
	t.Setenv("MSS_BOOT_FROM_UPPER", "upper-value")
	t.Setenv("mss_boot_from_lower", "lower-value")
	if err := os.Unsetenv("MSS_BOOT_ABSENT"); err != nil {
		t.Fatalf("Unsetenv() error = %v", err)
	}

	got := getValueFromEnv([]string{
		"Env.MSS_BOOT_DIRECT",
		"Env.mss_boot_from_upper",
		"Env.MSS_BOOT_FROM_LOWER",
		"Env.MSS_BOOT_ABSENT",
		"Other.IGNORED",
	})
	data, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("getValueFromEnv() = %T, want map[string]any", got)
	}
	env, ok := data["Env"].(map[string]string)
	if !ok {
		t.Fatalf("getValueFromEnv()[Env] = %T, want map[string]string", data["Env"])
	}

	assertEnvValue(t, env, "MSS_BOOT_DIRECT", "direct-value")
	assertEnvValue(t, env, "mss_boot_from_upper", "upper-value")
	assertEnvValue(t, env, "MSS_BOOT_FROM_LOWER", "lower-value")
	assertEnvValue(t, env, "MSS_BOOT_ABSENT", "")
	if _, exists := env["IGNORED"]; exists {
		t.Fatalf("non Env key should be ignored")
	}
}

func assertEnvValue(t *testing.T, env map[string]string, key, want string) {
	t.Helper()
	if got := env[key]; got != want {
		t.Fatalf("env[%q] = %q, want %q", key, got, want)
	}
}
