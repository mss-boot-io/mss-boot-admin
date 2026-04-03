package service

import (
	"github.com/mss-boot-io/mss-boot-admin/center/websocket"
	"github.com/mss-boot-io/mss-boot-admin/models"
)

type NotificationService struct{}

var Notification = &NotificationService{}

func (s *NotificationService) SendToUser(userID string, notice *models.Notice) bool {
	return websocket.SendNotificationToUser(userID, notice)
}

func (s *NotificationService) Broadcast(notice *models.Notice) {
	websocket.BroadcastNotification(notice)
}

func (s *NotificationService) KickUser(userID string, reason string) {
	websocket.SendKickToUser(userID, reason)
}

func (s *NotificationService) IsUserOnline(userID string) bool {
	return websocket.GetHub().IsUserOnline(userID)
}

func (s *NotificationService) GetOnlineInfo() map[string]interface{} {
	return websocket.GetOnlineInfo()
}
