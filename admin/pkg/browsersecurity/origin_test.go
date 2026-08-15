package browsersecurity

import "testing"

func TestNormalizeOrigin(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "https", raw: "https://ADMIN.example/", want: "https://admin.example", ok: true},
		{name: "default https port", raw: "https://admin.example:443", want: "https://admin.example", ok: true},
		{name: "custom port", raw: "http://localhost:8001", want: "http://localhost:8001", ok: true},
		{name: "ipv6", raw: "http://[::1]:8001", want: "http://[::1]:8001", ok: true},
		{name: "wildcard", raw: "*"},
		{name: "null", raw: "null"},
		{name: "credentials", raw: "https://user@admin.example"},
		{name: "path", raw: "https://admin.example/callback"},
		{name: "query", raw: "https://admin.example?next=/"},
		{name: "fragment", raw: "https://admin.example/#fragment"},
		{name: "javascript", raw: "javascript:alert(1)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := NormalizeOrigin(test.raw)
			if ok != test.ok || got != test.want {
				t.Fatalf("NormalizeOrigin(%q) = (%q, %v), want (%q, %v)", test.raw, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestIsTrustedOriginUsesExactConfiguredOrigins(t *testing.T) {
	cors := []string{"*", "https://admin.example/", "https://ADMIN.example", "http://localhost:8001"}
	if !IsTrustedOrigin("https://admin.example", "https://api.example", cors) {
		t.Fatal("configured frontend origin was rejected")
	}
	if !IsTrustedOrigin("https://api.example", "https://api.example/", cors) {
		t.Fatal("application origin was rejected")
	}
	for _, origin := range []string{"https://admin.example.attacker.test", "https://attacker.test", "null", ""} {
		if IsTrustedOrigin(origin, "https://api.example", cors) {
			t.Fatalf("untrusted origin %q was accepted", origin)
		}
	}
	want := []string{"https://admin.example", "http://localhost:8001"}
	got := TrustedOrigins(cors)
	if len(got) != len(want) {
		t.Fatalf("TrustedOrigins() = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("TrustedOrigins() = %#v, want %#v", got, want)
		}
	}
}
