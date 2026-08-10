package config

import (
	"encoding/json"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a strictly positive Go duration. Its zero value means that a
// field was omitted and lets Normalize apply the documented default.
// Configuration must use strings such as "5s"; numeric nanoseconds are never
// accepted.
type Duration struct {
	value time.Duration
}

// ParseDuration validates one explicit duration.
func ParseDuration(value string) (Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return Duration{}, invalid("duration", "must be a positive duration string")
	}
	return Duration{value: parsed}, nil
}

// Value returns the parsed duration and whether the field was explicitly set.
func (d Duration) Value() (time.Duration, bool) {
	return d.value, d.value != 0
}

func (d Duration) String() string {
	return d.value.String()
}

func (d Duration) MarshalJSON() ([]byte, error) {
	if d.value == 0 {
		return []byte(`""`), nil
	}
	return json.Marshal(d.value.String())
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return invalid("duration", "must be a positive duration string")
	}
	parsed, err := ParseDuration(value)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	if d.value == 0 {
		return "", nil
	}
	return d.value.String(), nil
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return invalid("duration", "must be a positive duration string")
	}
	parsed, err := ParseDuration(node.Value)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
