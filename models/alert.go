package models

import (
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

/*
 * Phase 4 - 告警规则配置
 * 用于监控系统指标并触发告警通知
 */

type AlertRule struct {
	actions.ModelGorm
	Name      string   `json:"name" gorm:"column:name;type:varchar(255);not null;comment:规则名称"`
	Metric    string   `json:"metric" gorm:"column:metric;type:varchar(50);not null;comment:监控指标"`
	Operator  string   `json:"operator" gorm:"column:operator;type:varchar(10);not null;comment:比较运算符"`
	Threshold float64  `json:"threshold" gorm:"column:threshold;type:decimal(10,2);not null;comment:阈值"`
	Duration  int      `json:"duration" gorm:"column:duration;type:int;default:60;comment:持续时间(秒)"`
	Channels  string   `json:"channels" gorm:"column:channels;type:text;comment:通知渠道(JSON数组)"`
	Message   string   `json:"message" gorm:"column:message;type:text;comment:告警消息模板"`
	Status    string   `json:"status" gorm:"column:status;type:varchar(20);default:'enabled';comment:状态"`
}

func (*AlertRule) TableName() string {
	return "mss_boot_alert_rules"
}

type AlertHistory struct {
	actions.ModelGorm
	RuleID      string  `json:"ruleId" gorm:"column:rule_id;type:varchar(36);not null;index;comment:规则ID"`
	RuleName    string  `json:"ruleName" gorm:"column:rule_name;type:varchar(255);comment:规则名称"`
	Metric      string  `json:"metric" gorm:"column:metric;type:varchar(50);comment:监控指标"`
	Value       float64 `json:"value" gorm:"column:value;type:decimal(10,2);comment:触发值"`
	Threshold   float64 `json:"threshold" gorm:"column:threshold;type:decimal(10,2);comment:阈值"`
	Status      string  `json:"status" gorm:"column:status;type:varchar(20);comment:状态(firing/resolved)"`
	TriggeredAt string  `json:"triggeredAt" gorm:"column:triggered_at;type:varchar(30);comment:触发时间"`
	ResolvedAt  string  `json:"resolvedAt" gorm:"column:resolved_at;type:varchar(30);comment:恢复时间"`
}

func (*AlertHistory) TableName() string {
	return "mss_boot_alert_histories"
}

const (
	AlertStatusEnabled  = "enabled"
	AlertStatusDisabled = "disabled"

	AlertStateFiring   = "firing"
	AlertStateResolved = "resolved"

	MetricCPU    = "cpu"
	MetricMemory = "memory"
	MetricDisk   = "disk"

	OperatorGT = ">"
	OperatorLT = "<"
	OperatorGE = ">="
	OperatorLE = "<="
)