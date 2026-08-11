package pkg

import (
	"strings"
	"testing"
)

func TestCanonicalEmailNormalizesBoundedASCIIIdentity(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: " Person+Tag@EXAMPLE.COM ", want: "person+tag@example.com", ok: true},
		{input: "person+tag@EXAMPLE.COM", want: "person+tag@example.com", ok: true},
		{input: "missing-at", ok: false},
		{input: "two@@example.com", ok: false},
		{input: "@example.com", ok: false},
		{input: "person@", ok: false},
		{input: "person@example.com\r\nBcc:x@example.com", ok: false},
		{input: ".person@example.com", ok: false},
		{input: "person..tag@example.com", ok: false},
		{input: "person@-example.com", ok: false},
		{input: "pérson@example.com", ok: false},
		{input: strings.Repeat("a", 90) + "@example.com", ok: false},
	}
	for _, test := range tests {
		got, ok := CanonicalEmail(test.input)
		if ok != test.ok || got != test.want {
			t.Errorf("CanonicalEmail(%q) = %q, %v; want %q, %v", test.input, got, ok, test.want, test.ok)
		}
	}

	upperLocal, _ := CanonicalEmail("Person@example.com")
	lowerLocal, _ := CanonicalEmail("person@example.com")
	if upperLocal != lowerLocal {
		t.Fatalf("case-equivalent email identities differ: %q != %q", upperLocal, lowerLocal)
	}
}
