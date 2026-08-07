package config

import "time"

type Monitor struct {
	SampleInterval time.Duration `yaml:"sampleInterval" json:"sampleInterval"`
	SampleTimeout  time.Duration `yaml:"sampleTimeout" json:"sampleTimeout"`
	HistorySize    int           `yaml:"historySize" json:"historySize"`
}
