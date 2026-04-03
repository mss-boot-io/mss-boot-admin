package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/config"
	"github.com/mss-boot-io/mss-boot-admin/models"
)

type NotificationChannel interface {
	Send(rule *models.AlertRule, value float64, message string) error
	Name() string
}

func GetEnabledChannels() []NotificationChannel {
	channels := []NotificationChannel{}
	cfg := config.Cfg.Notification

	if cfg.Email.Enabled && cfg.Email.Host != "" {
		channels = append(channels, &EmailChannel{cfg: cfg.Email})
	}
	if cfg.DingTalk.Enabled && cfg.DingTalk.Webhook != "" {
		channels = append(channels, &DingTalkChannel{cfg: cfg.DingTalk})
	}
	if cfg.WeChat.Enabled && cfg.WeChat.Webhook != "" {
		channels = append(channels, &WeChatChannel{cfg: cfg.WeChat})
	}
	return channels
}

func SendAlertToChannels(rule *models.AlertRule, value float64, message string) {
	channelList := strings.Split(rule.Channels, ",")
	if rule.Channels == "" {
		channelList = []string{"websocket"}
	}

	for _, chName := range channelList {
		switch strings.TrimSpace(chName) {
		case "email":
			if config.Cfg.Notification.Email.Enabled {
				ch := &EmailChannel{cfg: config.Cfg.Notification.Email}
				if err := ch.Send(rule, value, message); err != nil {
					slog.Error("failed to send email alert", "rule", rule.Name, "error", err)
				} else {
					slog.Info("email alert sent", "rule", rule.Name)
				}
			}
		case "dingtalk":
			if config.Cfg.Notification.DingTalk.Enabled {
				ch := &DingTalkChannel{cfg: config.Cfg.Notification.DingTalk}
				if err := ch.Send(rule, value, message); err != nil {
					slog.Error("failed to send dingtalk alert", "rule", rule.Name, "error", err)
				} else {
					slog.Info("dingtalk alert sent", "rule", rule.Name)
				}
			}
		case "wechat":
			if config.Cfg.Notification.WeChat.Enabled {
				ch := &WeChatChannel{cfg: config.Cfg.Notification.WeChat}
				if err := ch.Send(rule, value, message); err != nil {
					slog.Error("failed to send wechat alert", "rule", rule.Name, "error", err)
				} else {
					slog.Info("wechat alert sent", "rule", rule.Name)
				}
			}
		case "websocket":
		default:
		}
	}
}

type EmailChannel struct {
	cfg config.EmailConfig
}

func (c *EmailChannel) Name() string { return "email" }

func (c *EmailChannel) Send(rule *models.AlertRule, value float64, message string) error {
	if !c.cfg.Enabled || c.cfg.Host == "" {
		return nil
	}

	subject := fmt.Sprintf("[告警] %s - %s", rule.Name, rule.Metric)
	body := fmt.Sprintf("告警规则: %s\n监控指标: %s\n当前值: %.2f\n阈值: %.2f\n触发时间: %s\n\n%s",
		rule.Name, rule.Metric, value, rule.Threshold, time.Now().Format(time.RFC3339), message)

	auth := smtp.PlainAuth("", c.cfg.Username, c.cfg.Password, c.cfg.Host)
	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		c.cfg.From, c.cfg.Username, subject, body)

	return smtp.SendMail(addr, auth, c.cfg.From, []string{c.cfg.Username}, []byte(msg))
}

type DingTalkChannel struct {
	cfg config.DingTalkConfig
}

func (c *DingTalkChannel) Name() string { return "dingtalk" }

func (c *DingTalkChannel) Send(rule *models.AlertRule, value float64, message string) error {
	if !c.cfg.Enabled || c.cfg.Webhook == "" {
		return nil
	}

	webhook := c.cfg.Webhook
	if c.cfg.Secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := c.sign(timestamp, c.cfg.Secret)
		webhook = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhook, timestamp, sign)
	}

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": fmt.Sprintf("[告警] %s\n%s\n当前值: %.2f, 阈值: %.2f\n时间: %s",
				rule.Name, message, value, rule.Threshold, time.Now().Format(time.RFC3339)),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(webhook, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dingtalk returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *DingTalkChannel) sign(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

type WeChatChannel struct {
	cfg config.WeChatConfig
}

func (c *WeChatChannel) Name() string { return "wechat" }

func (c *WeChatChannel) Send(rule *models.AlertRule, value float64, message string) error {
	if !c.cfg.Enabled || c.cfg.Webhook == "" {
		return nil
	}

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": fmt.Sprintf("[告警] %s\n%s\n当前值: %.2f, 阈值: %.2f\n时间: %s",
				rule.Name, message, value, rule.Threshold, time.Now().Format(time.RFC3339)),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(c.cfg.Webhook, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wechat returned status %d", resp.StatusCode)
	}
	return nil
}