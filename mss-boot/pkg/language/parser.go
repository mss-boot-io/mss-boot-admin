package language

import (
	"sort"
	"strconv"
	"strings"
)

type language struct {
	name    string
	quality float64
}

type languageSlice []language

func (e languageSlice) SortByQuality() {
	sort.Stable(e)
}

func (e languageSlice) Len() int {
	return len(e)
}

func (e languageSlice) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e languageSlice) Less(i, j int) bool {
	return e[i].quality > e[j].quality
}

// ParseAcceptLanguage returns normalized RFC language codes ordered by quality.
// A quality of zero excludes an entry. When supportedLanguages is not empty,
// matching is case-insensitive and accepts underscores as hyphens.
func ParseAcceptLanguage(languages string, supportedLanguages []string) []string {
	preferred := strings.Split(languages, ",")
	supported := make(map[string]struct{}, len(supportedLanguages))
	for _, value := range supportedLanguages {
		value = normalizeLanguage(value)
		if value != "" {
			supported[value] = struct{}{}
		}
	}

	capacity := len(preferred)
	if len(supported) > 0 && len(supported) < capacity {
		capacity = len(supported)
	}
	languagesByQuality := make(languageSlice, 0, capacity)
	seen := make(map[string]struct{}, capacity)

	for index, raw := range preferred {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		parts := strings.SplitN(value, ";", 2)
		name := normalizeLanguage(parts[0])
		if name == "" {
			continue
		}
		if len(supported) > 0 {
			if _, ok := supported[name]; !ok {
				continue
			}
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}

		quality := float64(len(preferred) - index)
		if len(parts) == 2 {
			parameter := strings.TrimSpace(parts[1])
			if strings.HasPrefix(parameter, "q=") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(parameter, "q=")), 64)
				if err == nil && parsed >= 0 && parsed <= 1 {
					if parsed == 0 {
						continue
					}
					quality = parsed
				}
			}
		}

		seen[name] = struct{}{}
		languagesByQuality = append(languagesByQuality, language{name: name, quality: quality})
	}

	languagesByQuality.SortByQuality()
	result := make([]string, 0, len(languagesByQuality))
	for _, value := range languagesByQuality {
		result = append(result, value.name)
	}
	return result
}

func normalizeLanguage(value string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "_", "-")
}
