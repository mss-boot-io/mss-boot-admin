package spec

import (
	"sort"
	"strings"
	"unicode"
)

var identifierPattern = moduleNamePattern

func normalizeIdentifier(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func sortedStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func titleWords(value string) string {
	words := strings.FieldsFunc(value, func(r rune) bool {
		return r == '-' || r == '_' || unicode.IsSpace(r)
	})
	for index, word := range words {
		if word == "" {
			continue
		}
		runes := []rune(strings.ToLower(word))
		runes[0] = unicode.ToUpper(runes[0])
		words[index] = string(runes)
	}
	return strings.Join(words, " ")
}
