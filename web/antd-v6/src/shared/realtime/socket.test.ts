import { describe, expect, it, vi } from 'vitest';
import {
  APPLICATION_WEBSOCKET_PROTOCOL,
  connectRealtimeSocket,
  parseRealtimeMessage,
  REALTIME_EVENT_AUTHORIZATION,
  REALTIME_EVENT_CONNECTED,
  REALTIME_EVENT_KICK,
  REALTIME_EVENT_PING,
  REALTIME_EVENT_PONG,
  realtimeReconnectDelay,
  realtimeWebSocketProtocols,
  realtimeWebSocketURL,
  startAuthorizationRealtimeSession,
  WEBSOCKET_CONNECT_PATH,
  WEBSOCKET_TICKET_PROTOCOL_PREFIX,
} from './socket';

const ticket = 'a'.repeat(43);

class FakeSocket {
  readyState = 1;
  sent: string[] = [];
  closeCalls: Array<[number | undefined, string | undefined]> = [];
  onopen: WebSocket['onopen'] = null;
  onmessage: WebSocket['onmessage'] = null;
  onerror: WebSocket['onerror'] = null;
  onclose: WebSocket['onclose'] = null;

  send(value: string) {
    this.sent.push(value);
  }

  close(code?: number, reason?: string) {
    this.readyState = 3;
    this.closeCalls.push([code, reason]);
  }

  message(value: unknown) {
    const handler = this.onmessage as ((event: MessageEvent) => void) | null;
    handler?.({ data: value } as MessageEvent);
  }

  serverClose() {
    this.readyState = 3;
    const handler = this.onclose as ((event: CloseEvent) => void) | null;
    handler?.(new CloseEvent('close'));
  }
}

describe('realtime WebSocket transport', () => {
  it('keeps the credential out of the URL and switches schemes safely', () => {
    expect(realtimeWebSocketURL({ protocol: 'https:', host: 'admin.example.test' })).toBe(
      `wss://admin.example.test${WEBSOCKET_CONNECT_PATH}`,
    );
    expect(realtimeWebSocketURL({ protocol: 'http:', host: 'localhost:8001' })).toBe(
      `ws://localhost:8001${WEBSOCKET_CONNECT_PATH}`,
    );
    expect(realtimeWebSocketURL({ protocol: 'https:', host: 'admin.example.test' })).not.toContain(
      ticket,
    );
  });

  it('carries a validated single-use ticket only as a subprotocol', () => {
    expect(
      realtimeWebSocketProtocols({
        ticket,
        protocol: APPLICATION_WEBSOCKET_PROTOCOL,
        expiresAt: new Date().toISOString(),
      }),
    ).toEqual([APPLICATION_WEBSOCKET_PROTOCOL, `${WEBSOCKET_TICKET_PROTOCOL_PREFIX}${ticket}`]);
    expect(() =>
      realtimeWebSocketProtocols({
        ticket: 'bad.ticket',
        protocol: APPLICATION_WEBSOCKET_PROTOCOL,
        expiresAt: new Date().toISOString(),
      }),
    ).toThrow(/Invalid realtime ticket/);
  });

  it('issues the ticket before creating the browser socket', async () => {
    const issueTicket = vi.fn(async () => ({
      ticket,
      protocol: APPLICATION_WEBSOCKET_PROTOCOL,
      expiresAt: new Date().toISOString(),
    }));
    const socket = {} as WebSocket;
    const socketFactory = vi.fn((_url: string, _protocols: string[]) => socket);

    await expect(
      connectRealtimeSocket({
        issueTicket,
        location: { protocol: 'https:', host: 'admin.example.test' },
        socketFactory,
      }),
    ).resolves.toBe(socket);
    expect(issueTicket).toHaveBeenCalledOnce();
    expect(socketFactory).toHaveBeenCalledWith(
      `wss://admin.example.test${WEBSOCKET_CONNECT_PATH}`,
      [APPLICATION_WEBSOCKET_PROTOCOL, `${WEBSOCKET_TICKET_PROTOCOL_PREFIX}${ticket}`],
    );
    expect(socketFactory.mock.calls[0]?.[0]).not.toContain(ticket);
  });

  it('strictly parses server events and keeps revisions as decimal strings', () => {
    expect(
      parseRealtimeMessage(
        JSON.stringify({
          event: REALTIME_EVENT_AUTHORIZATION,
          code: 200,
          data: { revision: '18446744073709551615' },
        }),
      ),
    ).toEqual({ type: 'authorization', revision: '18446744073709551615' });
    expect(
      parseRealtimeMessage(
        JSON.stringify({
          event: REALTIME_EVENT_AUTHORIZATION,
          code: 200,
          data: { revision: 42 },
        }),
      ),
    ).toBeUndefined();
    expect(parseRealtimeMessage('{bad-json')).toBeUndefined();
    expect(parseRealtimeMessage(new ArrayBuffer(1))).toBeUndefined();
    expect(parseRealtimeMessage(JSON.stringify({ event: 'unknown', code: 200 }))).toBeUndefined();
  });

  it('uses bounded exponential reconnect delay with additive jitter', () => {
    expect(realtimeReconnectDelay(0, () => 0)).toBe(1_000);
    expect(realtimeReconnectDelay(2, () => 0.5)).toBe(4_400);
    expect(realtimeReconnectDelay(99, () => 1)).toBe(30_000);
  });

  it('answers heartbeat, forwards revision hints, and stops after a kick', async () => {
    const socket = new FakeSocket();
    const onAuthorizationRevision = vi.fn();
    const onKick = vi.fn();
    const onReconnected = vi.fn();
    const stop = startAuthorizationRealtimeSession({
      connect: vi.fn(async () => socket as unknown as WebSocket),
      onAuthorizationRevision,
      onKick,
      onReconnected,
      onlineTarget: new EventTarget(),
      visibilityTarget: new EventTarget(),
      isOnline: () => true,
      isVisible: () => true,
    });
    await vi.waitFor(() => expect(socket.onmessage).toBeTypeOf('function'));

    socket.message(JSON.stringify({ event: REALTIME_EVENT_CONNECTED, code: 200 }));
    socket.message(JSON.stringify({ event: REALTIME_EVENT_CONNECTED, code: 200 }));
    expect(onReconnected).not.toHaveBeenCalled();
    socket.message(JSON.stringify({ event: REALTIME_EVENT_PING, code: 0 }));
    expect(socket.sent).toEqual([JSON.stringify({ event: REALTIME_EVENT_PONG })]);

    socket.message(
      JSON.stringify({
        event: REALTIME_EVENT_AUTHORIZATION,
        code: 200,
        data: { revision: '7' },
      }),
    );
    expect(onAuthorizationRevision).toHaveBeenCalledWith('7');

    socket.message(
      JSON.stringify({ event: REALTIME_EVENT_KICK, code: 200, data: { reason: 'revoked' } }),
    );
    expect(onKick).toHaveBeenCalledWith('revoked');
    expect(socket.closeCalls).toEqual([[1000, 'client shutdown']]);
    stop();
  });

  it('refreshes authority once after an acknowledged connection is replaced', async () => {
    vi.useFakeTimers();
    try {
      const first = new FakeSocket();
      const second = new FakeSocket();
      const connect = vi
        .fn<() => Promise<WebSocket>>()
        .mockResolvedValueOnce(first as unknown as WebSocket)
        .mockResolvedValueOnce(second as unknown as WebSocket);
      const onReconnected = vi.fn();
      const stop = startAuthorizationRealtimeSession({
        connect,
        onAuthorizationRevision: vi.fn(),
        onKick: vi.fn(),
        onReconnected,
        onlineTarget: new EventTarget(),
        visibilityTarget: new EventTarget(),
        isOnline: () => true,
        isVisible: () => true,
        random: () => 0,
      });
      await Promise.resolve();
      first.message(JSON.stringify({ event: REALTIME_EVENT_CONNECTED, code: 200 }));

      first.serverClose();
      await vi.advanceTimersByTimeAsync(1_000);
      expect(connect).toHaveBeenCalledTimes(2);
      second.message(JSON.stringify({ event: REALTIME_EVENT_CONNECTED, code: 200 }));
      second.message(JSON.stringify({ event: REALTIME_EVENT_CONNECTED, code: 200 }));
      expect(onReconnected).toHaveBeenCalledOnce();
      stop();
    } finally {
      vi.useRealTimers();
    }
  });
});
