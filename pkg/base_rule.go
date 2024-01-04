package pkg

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/4 14:47:51
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/4 14:47:51
 */

type BaseRule struct {
	ID              string   `json:"id"`
	WarningOnly     bool     `json:"warningOnly,omitempty"`
	Len             uint8    `json:"len,omitempty"`
	Max             uint8    `json:"max,omitempty"`
	Min             uint8    `json:"min,omitempty"`
	Message         string   `json:"message,omitempty"`
	Pattern         string   `json:"pattern,omitempty"`
	Required        bool     `json:"required,omitempty"`
	Type            RuleType `json:"type,omitempty"`
	Whitespace      bool     `json:"whitespace,omitempty"`
	ValidateTrigger string   `json:"validateTrigger,omitempty"`
}

type RuleType string

const (
	RUleTypeString RuleType = "string"
	RUleTypeNumber RuleType = "number"
	RUleTypeBool   RuleType = "boolean"
	RUleTypeMethod RuleType = "method"
	RUleTypeRegexp RuleType = "regexp"
	RUleTypeInt    RuleType = "integer"
	RUleTypeFloat  RuleType = "float"
	RUleTypeObject RuleType = "object"
	RUleTypeEnum   RuleType = "enum"
	RUleTypeDate   RuleType = "date"
	RUleTypeUrl    RuleType = "url"
	RUleTypeHex    RuleType = "hex"
	RUleTypeEmail  RuleType = "email"
)
