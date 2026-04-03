package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/center/websocket"
	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"gorm.io/gorm"
)

type AlertChecker struct {
	db        *gorm.DB
	ticker    *time.Ticker
	stopChan  chan struct{}
	interval  time.Duration
}

var alertChecker *AlertChecker

func InitAlertChecker(db *gorm.DB, interval time.Duration) {
	if alertChecker != nil {
		return
	}
	alertChecker = &AlertChecker{
		db:       db,
		interval: interval,
		stopChan: make(chan struct{}),
	}
	go alertChecker.Run()
}

func StopAlertChecker() {
	if alertChecker != nil {
		close(alertChecker.stopChan)
	}
}

func (a *AlertChecker) Run() {
	a.ticker = time.NewTicker(a.interval)
	defer a.ticker.Stop()

	for {
		select {
		case <-a.ticker.C:
			a.check()
		case <-a.stopChan:
			return
		}
	}
}

func (a *AlertChecker) check() {
	var rules []*models.AlertRule
	if err := a.db.Where("status = ?", models.AlertStatusEnabled).Find(&rules).Error; err != nil {
		slog.Error("failed to get alert rules", "error", err)
		return
	}

	for _, rule := range rules {
		value, err := a.getMetricValue(rule.Metric)
		if err != nil {
			slog.Error("failed to get metric value", "metric", rule.Metric, "error", err)
			continue
		}

		if a.evaluateRule(rule, value) {
			a.triggerAlert(rule, value)
		}
	}
}

func (a *AlertChecker) getMetricValue(metric string) (float64, error) {
	ctx := context.Background()

	switch metric {
	case models.MetricCPU:
		percent, err := cpu.PercentWithContext(ctx, 1*time.Second, false)
		if err != nil {
			return 0, err
		}
		if len(percent) > 0 {
			return percent[0], nil
		}
		return 0, nil
	case models.MetricMemory:
		memInfo, err := mem.VirtualMemoryWithContext(ctx)
		if err != nil {
			return 0, err
		}
		return memInfo.UsedPercent, nil
	case models.MetricDisk:
		diskInfo, err := disk.UsageWithContext(ctx, "/")
		if err != nil {
			return 0, err
		}
		return diskInfo.UsedPercent, nil
	default:
		return 0, fmt.Errorf("unknown metric: %s", metric)
	}
}

func (a *AlertChecker) evaluateRule(rule *models.AlertRule, value float64) bool {
	switch rule.Operator {
	case models.OperatorGT:
		return value > rule.Threshold
	case models.OperatorGE:
		return value >= rule.Threshold
	case models.OperatorLT:
		return value < rule.Threshold
	case models.OperatorLE:
		return value <= rule.Threshold
	default:
		return false
	}
}

func (a *AlertChecker) triggerAlert(rule *models.AlertRule, value float64) {
	now := time.Now().Format(time.RFC3339)

	history := &models.AlertHistory{
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		Metric:      rule.Metric,
		Value:       value,
		Threshold:   rule.Threshold,
		Status:      models.AlertStateFiring,
		TriggeredAt: now,
	}

	if err := a.db.Create(history).Error; err != nil {
		slog.Error("failed to create alert history", "error", err)
		return
	}

	message := fmt.Sprintf("[告警] %s: %s 当前值 %.2f 超过阈值 %.2f",
		rule.Name, rule.Metric, value, rule.Threshold)
	if rule.Message != "" {
		message = rule.Message
		message = fmt.Sprintf(message, value, rule.Threshold)
	}

	websocket.GetHub().Broadcast(&websocket.WResponse{
		Event:     websocket.EventNotify,
		Code:      200,
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"type":      "alert",
			"ruleId":    rule.ID,
			"ruleName":  rule.Name,
			"metric":    rule.Metric,
			"value":     value,
			"threshold": rule.Threshold,
			"message":   message,
		},
	})

	SendAlertToChannels(rule, value, message)

	slog.Info("alert triggered",
		"rule", rule.Name,
		"metric", rule.Metric,
		"value", value,
		"threshold", rule.Threshold)
}