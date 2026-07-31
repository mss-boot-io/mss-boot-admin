package model

import (
	mgm "github.com/kamva/mgm/v3"
	"gorm.io/gorm/schema"
)

// ModelProvider model provider
type ModelProvider string

const (
	// ModelProviderMgm mgm model provider
	ModelProviderMgm ModelProvider = "mgm"
	// ModelProviderGorm gorm model provider
	ModelProviderGorm ModelProvider = "gorm"
)

// ModelImpl gorm and mgm model
type ModelImpl interface {
	mgm.Model
	schema.Tabler
}

func (p ModelProvider) String() string {
	return string(p)
}
