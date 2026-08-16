import { App } from 'antd';
import type { MessageInstance } from 'antd/es/message/interface';
import type { NotificationInstance } from 'antd/es/notification/interface';
import { useEffect } from 'react';

interface FeedbackApi {
  message: MessageInstance;
  notification: NotificationInstance;
}

let activeFeedback: FeedbackApi | undefined;

export function RuntimeFeedbackBridge() {
  const { message, notification } = App.useApp();

  useEffect(() => {
    activeFeedback = { message, notification };
    return () => {
      activeFeedback = undefined;
    };
  }, [message, notification]);

  return null;
}

export function feedback(): FeedbackApi | undefined {
  return activeFeedback;
}
