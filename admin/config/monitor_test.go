package config

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestMonitorConfigParsesDurationValues(t *testing.T) {
	var value struct {
		Monitor Monitor `yaml:"monitor"`
	}
	err := yaml.Unmarshal([]byte(`
monitor:
  sampleInterval: 5s
  sampleTimeout: 3s
  historySize: 120
`), &value)
	if err != nil {
		t.Fatalf("unmarshal monitor config: %v", err)
	}
	if value.Monitor.SampleInterval != 5*time.Second ||
		value.Monitor.SampleTimeout != 3*time.Second ||
		value.Monitor.HistorySize != 120 {
		t.Fatalf("monitor config = %#v", value.Monitor)
	}
}
