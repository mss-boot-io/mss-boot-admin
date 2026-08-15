import { describe, expect, it } from 'vitest';
import { ONLINE_SESSION_REFRESH_MS, onlineSessionRefetchInterval } from './polling';

describe('online session polling', () => {
  it('stops automatic requests when authentication or authorization is no longer valid', () => {
    expect(onlineSessionRefetchInterval({ response: { status: 401 } })).toBe(false);
    expect(onlineSessionRefetchInterval({ response: { status: 403 } })).toBe(false);
  });

  it('keeps the bounded foreground cadence for successful and transient states', () => {
    expect(onlineSessionRefetchInterval(undefined)).toBe(ONLINE_SESSION_REFRESH_MS);
    expect(onlineSessionRefetchInterval({ response: { status: 503 } })).toBe(
      ONLINE_SESSION_REFRESH_MS,
    );
  });
});
