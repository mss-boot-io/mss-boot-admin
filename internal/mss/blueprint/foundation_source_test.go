package blueprint

import (
	"strconv"
	"testing"
)

func TestParseCommittedBlobSizeUsesNativeIntWidth(t *testing.T) {
	maxValue := "2147483647"
	overflowValue := "2147483648"
	if strconv.IntSize == 64 {
		maxValue = "9223372036854775807"
		overflowValue = "9223372036854775808"
	}
	tests := []struct {
		name      string
		value     string
		wantValue string
		ok        bool
	}{
		{name: "zero", value: "0", wantValue: "0", ok: true},
		{name: "small", value: "42", wantValue: "42", ok: true},
		{name: "native maximum", value: maxValue, wantValue: maxValue, ok: true},
		{name: "native overflow", value: overflowValue, ok: false},
		{name: "negative", value: "-1", ok: false},
		{name: "malformed", value: "not-a-size", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := parseCommittedBlobSize(test.value)
			if ok != test.ok {
				t.Fatalf("parseCommittedBlobSize(%q) ok = %v, want %v", test.value, ok, test.ok)
			}
			if ok && strconv.FormatInt(int64(got), 10) != test.wantValue {
				t.Fatalf("parseCommittedBlobSize(%q) value = %d, want %s", test.value, got, test.wantValue)
			}
			if !ok && got != 0 {
				t.Fatalf("parseCommittedBlobSize(%q) value = %d on failure, want zero", test.value, got)
			}
		})
	}
}
