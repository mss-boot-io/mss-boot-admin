import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { useWebSocket, WebSocketMessage } from '@/hooks/useWebSocket';
import { getNoticeUnread, putNoticeReadId } from '@/services/admin/notice';
import { message } from 'antd';

interface Notification {
  id: string;
  type: string;
  title: string;
  description?: string;
  createdAt?: string;
  read?: boolean;
}

interface NotificationContextType {
  notifications: Notification[];
  unreadCount: number;
  isConnected: boolean;
  markAsRead: (id: string) => Promise<void>;
  markAllAsRead: () => Promise<void>;
  refreshUnread: () => Promise<void>;
}

const NotificationContext = createContext<NotificationContextType | undefined>(undefined);

export const NotificationProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [notifications, setNotifications] = useState<Notification[]>([]);

  const wsUrl = (() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    return `${protocol}//${host}/admin/api/ws/connect`;
  })();

  const handleWebSocketMessage = useCallback((msg: WebSocketMessage) => {
    if (msg.event === 'notify' && msg.code === 200) {
      const notification = msg.data as Notification;
      setNotifications((prev) => [notification, ...prev]);

      message.info({
        content: notification.title,
        duration: 5,
      });
    }
  }, []);

  const { isConnected } = useWebSocket({
    url: wsUrl,
    onMessage: handleWebSocketMessage,
    reconnect: true,
  });

  const refreshUnread = useCallback(async () => {
    try {
      const res = await getNoticeUnread();
      if (res) {
        setNotifications(res as Notification[]);
      }
    } catch (error) {
      console.error('Failed to fetch unread notifications:', error);
    }
  }, []);

  const markAsRead = useCallback(async (id: string) => {
    try {
      await putNoticeReadId({ id });
      setNotifications((prev) => prev.filter((n) => n.id !== id));
    } catch (error) {
      console.error('Failed to mark notification as read:', error);
    }
  }, []);

  const markAllAsRead = useCallback(async () => {
    try {
      await putNoticeReadId({ id: 'all' });
      setNotifications([]);
    } catch (error) {
      console.error('Failed to mark all notifications as read:', error);
    }
  }, []);

  useEffect(() => {
    refreshUnread();
  }, [refreshUnread]);

  const unreadCount = notifications.length;

  return (
    <NotificationContext.Provider
      value={{
        notifications,
        unreadCount,
        isConnected,
        markAsRead,
        markAllAsRead,
        refreshUnread,
      }}
    >
      {children}
    </NotificationContext.Provider>
  );
};

export const useNotification = () => {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error('useNotification must be used within a NotificationProvider');
  }
  return context;
};

export default NotificationContext;
