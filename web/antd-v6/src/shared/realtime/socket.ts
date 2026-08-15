import { request } from '@umijs/max';

export const APPLICATION_WEBSOCKET_PROTOCOL = 'mss.v1';
export const WEBSOCKET_TICKET_PROTOCOL_PREFIX = 'mss.ticket.';
export const WEBSOCKET_TICKET_ENDPOINT = '/ws/tickets';
export const WEBSOCKET_CONNECT_PATH = '/admin/api/ws/connect-v6';

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
