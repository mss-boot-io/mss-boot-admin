import { request } from '@umijs/max';

export const APPLICATION_WEBSOCKET_PROTOCOL = 'mss.v1';
export const WEBSOCKET_TICKET_PROTOCOL_PREFIX = 'mss.ticket.';
export const WEBSOCKET_TICKET_ENDPOINT = '/ws/tickets';
export const WEBSOCKET_CONNECT_PATH = '/admin/api/ws/connect-v6';
export const REALTIME_EVENT_CONNECTED = 'connected';
export const REALTIME_EVENT_PING = 'ping';
export const REALTIME_EVENT_PONG = 'pong';
export const REALTIME_EVENT_AUTHORIZATION = 'authorization';
export const REALTIME_EVENT_KICK = 'kick';
export const REALTIME_RECONNECT_MAX_DELAY_MS = 30_000;

export interface WebSocketTicketResponse {
  ticket: string;
  protocol: string;
  expiresAt: string;
}

interface BrowserLocation {
  host: string;
  protocol: string;
}

interface ConnectRealtimeOptions {
  issueTicket?: () => Promise<WebSocketTicketResponse>;
  location?: BrowserLocation;
  socketFactory?: (url: string, protocols: string[]) => WebSocket;
}

type ParsedRealtimeMessage =
  | { type: 'connected' }
  | { type: 'ping' }
  | { type: 'authorization'; revision: string }
  | { type: 'kick'; reason?: string };

export interface AuthorizationRealtimeSessionOptions {
  connect?: () => Promise<WebSocket>;
  onAuthorizationRevision: (revision: string) => void;
  onKick: (reason?: string) => void;
  onReconnected?: () => void;
  shouldReconnect?: (error: unknown) => boolean;
  onlineTarget?: EventTarget;
  visibilityTarget?: EventTarget;
  isOnline?: () => boolean;
  isVisible?: () => boolean;
  random?: () => number;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function canonicalRevision(value: unknown): value is string {
  return typeof value === 'string' && /^[1-9]\d*$/.test(value);
}

export function parseRealtimeMessage(value: unknown): ParsedRealtimeMessage | undefined {
  if (typeof value !== 'string') return undefined;
  let envelope: unknown;
  try {
    envelope = JSON.parse(value);
  } catch {
    return undefined;
  }
  if (!isRecord(envelope) || typeof envelope.event !== 'string') return undefined;
  switch (envelope.event) {
    case REALTIME_EVENT_CONNECTED:
      return envelope.code === 200 ? { type: 'connected' } : undefined;
    case REALTIME_EVENT_PING:
      return { type: 'ping' };
    case REALTIME_EVENT_AUTHORIZATION: {
      if (envelope.code !== 200 || !isRecord(envelope.data)) return undefined;
      const revision = envelope.data.revision;
      return canonicalRevision(revision) ? { type: 'authorization', revision } : undefined;
    }
    case REALTIME_EVENT_KICK: {
      if (envelope.code !== 200) return undefined;
      if (!isRecord(envelope.data) || typeof envelope.data.reason !== 'string') {
        return { type: 'kick' };
      }
      const reason = envelope.data.reason.trim().slice(0, 256);
      return reason ? { type: 'kick', reason } : { type: 'kick' };
    }
    default:
      return undefined;
  }
}

export function realtimeReconnectDelay(attempt: number, random = Math.random): number {
  const exponent = Math.min(Math.max(0, Math.floor(attempt)), 5);
  const base = Math.min(REALTIME_RECONNECT_MAX_DELAY_MS, 1_000 * 2 ** exponent);
  const jitterRatio = Math.min(1, Math.max(0, random()));
  return Math.min(REALTIME_RECONNECT_MAX_DELAY_MS, base + Math.floor(base * 0.2 * jitterRatio));
}

export function realtimeWebSocketURL(location: BrowserLocation = window.location): string {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${protocol}//${location.host}${WEBSOCKET_CONNECT_PATH}`;
}

export function realtimeWebSocketProtocols(ticket: WebSocketTicketResponse): string[] {
  if (ticket.protocol !== APPLICATION_WEBSOCKET_PROTOCOL) {
    throw new Error('Unsupported realtime application protocol');
  }
  if (!/^[A-Za-z0-9_-]{43}$/.test(ticket.ticket)) {
    throw new Error('Invalid realtime ticket');
  }
  return [APPLICATION_WEBSOCKET_PROTOCOL, `${WEBSOCKET_TICKET_PROTOCOL_PREFIX}${ticket.ticket}`];
}

async function issueRealtimeTicket(): Promise<WebSocketTicketResponse> {
  return request<WebSocketTicketResponse>(WEBSOCKET_TICKET_ENDPOINT, {
    method: 'POST',
    skipErrorHandler: true,
    skipAuthorizationRefresh: true,
  });
}

export async function connectRealtimeSocket(
  options: ConnectRealtimeOptions = {},
): Promise<WebSocket> {
  const ticket = await (options.issueTicket ?? issueRealtimeTicket)();
  const url = realtimeWebSocketURL(options.location);
  const protocols = realtimeWebSocketProtocols(ticket);
  const socketFactory =
    options.socketFactory ??
    ((socketURL, socketProtocols) => new WebSocket(socketURL, socketProtocols));
  return socketFactory(url, protocols);
}

/**
 * Owns one authenticated realtime connection. Revision messages are hints;
 * the caller decides how to reload authoritative identity and menu state.
 */
export function startAuthorizationRealtimeSession(
  options: AuthorizationRealtimeSessionOptions,
): () => void {
  const connect = options.connect ?? (() => connectRealtimeSocket());
  const onlineTarget = options.onlineTarget ?? window;
  const visibilityTarget = options.visibilityTarget ?? document;
  const isOnline = options.isOnline ?? (() => navigator.onLine);
  const isVisible = options.isVisible ?? (() => document.visibilityState === 'visible');
  const random = options.random ?? Math.random;
  let activeSocket: WebSocket | undefined;
  let connecting = false;
  let stopped = false;
  let acknowledged = false;
  let reconnectAttempt = 0;
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined;

  const clearReconnect = () => {
    if (reconnectTimer === undefined) return;
    clearTimeout(reconnectTimer);
    reconnectTimer = undefined;
  };

  const closeActiveSocket = () => {
    const socket = activeSocket;
    activeSocket = undefined;
    if (!socket) return;
    socket.onopen = null;
    socket.onmessage = null;
    socket.onerror = null;
    socket.onclose = null;
    try {
      socket.close(1000, 'client shutdown');
    } catch {
      // The socket may still be transitioning between CONNECTING and OPEN.
    }
  };

  const stop = () => {
    if (stopped) return;
    stopped = true;
    clearReconnect();
    onlineTarget.removeEventListener('online', reconnectNow);
    visibilityTarget.removeEventListener('visibilitychange', reconnectWhenVisible);
    closeActiveSocket();
  };

  const scheduleReconnect = () => {
    if (stopped || connecting || activeSocket || reconnectTimer !== undefined || !isOnline())
      return;
    const delay = realtimeReconnectDelay(reconnectAttempt, random);
    reconnectAttempt += 1;
    reconnectTimer = setTimeout(() => {
      reconnectTimer = undefined;
      void open();
    }, delay);
  };

  const handleConnectionFailure = (error: unknown) => {
    if (options.shouldReconnect?.(error) === false) {
      stop();
      return;
    }
    scheduleReconnect();
  };

  const open = async () => {
    if (stopped || connecting || activeSocket || !isOnline()) return;
    connecting = true;
    let socket: WebSocket;
    try {
      socket = await connect();
    } catch (error) {
      connecting = false;
      handleConnectionFailure(error);
      return;
    }
    connecting = false;
    if (stopped) {
      try {
        socket.close(1000, 'client shutdown');
      } catch {
        // Nothing else owns this late connection.
      }
      return;
    }

    activeSocket = socket;
    let connectionAcknowledged = false;
    socket.onmessage = (event) => {
      const message = parseRealtimeMessage(event.data);
      if (!message) return;
      switch (message.type) {
        case 'connected': {
          if (connectionAcknowledged) return;
          connectionAcknowledged = true;
          const reconnected = acknowledged;
          acknowledged = true;
          reconnectAttempt = 0;
          if (reconnected) options.onReconnected?.();
          return;
        }
        case 'ping':
          if (socket.readyState === 1) {
            socket.send(JSON.stringify({ event: REALTIME_EVENT_PONG }));
          }
          return;
        case 'authorization':
          options.onAuthorizationRevision(message.revision);
          return;
        case 'kick':
          stop();
          options.onKick(message.reason);
          return;
      }
    };
    socket.onerror = () => {
      if (activeSocket !== socket) return;
      activeSocket = undefined;
      try {
        socket.close();
      } catch {
        // onclose may not follow a failed CONNECTING socket.
      }
      scheduleReconnect();
    };
    socket.onclose = () => {
      if (activeSocket === socket) activeSocket = undefined;
      scheduleReconnect();
    };
  };

  function reconnectNow() {
    if (stopped || !isOnline()) return;
    clearReconnect();
    if (!activeSocket && !connecting) void open();
  }

  function reconnectWhenVisible() {
    if (isVisible()) reconnectNow();
  }

  onlineTarget.addEventListener('online', reconnectNow);
  visibilityTarget.addEventListener('visibilitychange', reconnectWhenVisible);
  void open();
  return stop;
}
