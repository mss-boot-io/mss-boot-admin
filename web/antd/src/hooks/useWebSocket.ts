import { useCallback, useEffect, useRef, useState } from 'react';
import { message } from 'antd';

export interface WebSocketMessage {
  event: string;
  data?: any;
  code: number;
  errorMsg?: string;
  timestamp: number;
}

export interface WebSocketOptions {
  url: string;
  onMessage?: (msg: WebSocketMessage) => void;
  onOpen?: () => void;
  onClose?: () => void;
  onError?: (error: Event) => void;
  reconnect?: boolean;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
  heartbeatInterval?: number;
}

export const useWebSocket = (options: WebSocketOptions) => {
  const {
    url,
    onMessage,
    onOpen,
    onClose,
    onError,
    reconnect = true,
    reconnectInterval = 3000,
    maxReconnectAttempts = 5,
    heartbeatInterval = 30000,
  } = options;

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const heartbeatTimerRef = useRef<NodeJS.Timeout>();
  const [isConnected, setIsConnected] = useState(false);

  const send = useCallback((event: string, data?: any) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ event, data }));
    }
  }, []);

  const connect = useCallback(() => {
    const token = localStorage.getItem('token');
    if (!token) {
      return;
    }

    const wsUrl = `${url}?token=${token}`;

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      setIsConnected(true);
      reconnectAttemptsRef.current = 0;
      onOpen?.();

      heartbeatTimerRef.current = setInterval(() => {
        send('ping');
      }, heartbeatInterval);
    };

    ws.onmessage = (event) => {
      try {
        const msg: WebSocketMessage = JSON.parse(event.data);

        if (msg.event === 'ping') {
          send('pong');
          return;
        }

        if (msg.event === 'kick') {
          message.warning(msg.data?.reason || 'You have been kicked out');
          ws.close();
          return;
        }

        onMessage?.(msg);
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e);
      }
    };

    ws.onclose = () => {
      setIsConnected(false);
      wsRef.current = null;

      if (heartbeatTimerRef.current) {
        clearInterval(heartbeatTimerRef.current);
      }

      onClose?.();

      if (reconnect && reconnectAttemptsRef.current < maxReconnectAttempts) {
        reconnectAttemptsRef.current++;
        setTimeout(connect, reconnectInterval);
      }
    };

    ws.onerror = (error) => {
      onError?.(error);
    };
  }, [
    url,
    onMessage,
    onOpen,
    onClose,
    onError,
    reconnect,
    reconnectInterval,
    maxReconnectAttempts,
    heartbeatInterval,
    send,
  ]);

  const disconnect = useCallback(() => {
    if (heartbeatTimerRef.current) {
      clearInterval(heartbeatTimerRef.current);
    }
    wsRef.current?.close();
    wsRef.current = null;
    setIsConnected(false);
  }, []);

  useEffect(() => {
    connect();
    return () => {
      disconnect();
    };
  }, [connect, disconnect]);

  return {
    isConnected,
    send,
    disconnect,
    reconnect: connect,
  };
};

export default useWebSocket;
