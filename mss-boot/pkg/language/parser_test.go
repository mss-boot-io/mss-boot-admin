package language

import (
	"reflect"
	"testing"
)

func TestParseAcceptLanguageOrdersFiltersAndNormalizes(t *testing.T) {
	got := ParseAcceptLanguage(
		" fr-CA;q=0.7, EN_us;q=0.9, de;q=0, en-US;q=0.8, invalid;q=bad ",
		[]string{"EN-US", "fr_ca", "de", "invalid"},
	)
	want := []string{"en-us", "fr-ca"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("languages = %#v, want %#v", got, want)
	}
}

func TestParseAcceptLanguagePreservesOrderWithoutQuality(t *testing.T) {
	got := ParseAcceptLanguage("zh-CN,en-US,fr-FR", nil)
	want := []string{"zh-cn", "en-us", "fr-fr"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("languages = %#v, want %#v", got, want)
	}
}

func TestParseAcceptLanguageDeduplicatesAndSkipsInvalidValues(t *testing.T) {
	got := ParseAcceptLanguage(" ,EN-US,en_us;q=0.2,,fr;q=0,de;q=1.5,it;q=-1 ", nil)
	want := []string{"en-us"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("languages = %#v, want %#v", got, want)
	}
}

func TestLanguageSliceSortContract(t *testing.T) {
	values := languageSlice{
		{name: "low", quality: 0.1},
		{name: "high", quality: 1},
		{name: "middle", quality: 0.5},
	}
	if values.Len() != 3 || values.Less(0, 1) {
		t.Fatalf("initial sort contract = %#v", values)
	}
	values.Swap(0, 1)
	if values[0].name != "high" {
		t.Fatal("Swap did not exchange entries")
	}
	values.SortByQuality()
	if values[0].name != "high" || values[2].name != "low" {
		t.Fatalf("sorted values = %#v", values)
	}
}
