import { describe, expect, it, vi } from 'vitest';
import {
  APPLICATION_WEBSOCKET_PROTOCOL,
  connectRealtimeSocket,
  realtimeWebSocketProtocols,
  realtimeWebSocketURL,
  WEBSOCKET_CONNECT_PATH,
  WEBSOCKET_TICKET_PROTOCOL_PREFIX,
} from './socket';

const ticket = 'a'.repeat(43);

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
});
