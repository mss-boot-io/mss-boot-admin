import { describe, expect, it } from 'vitest';
import { monitorRefetchInterval, monitorRetryAfterDelay } from './polling';

describe('monitor polling policy', () => {
  it('honors bounded Retry-After and stops terminal authorization polling', () => {
    const unavailable = {
      response: { status: 503, headers: new Headers({ 'Retry-After': '12' }) },
    };
    expect(monitorRetryAfterDelay(unavailable, 0)).toBe(12_000);
    expect(monitorRefetchInterval(undefined, unavailable, 1)).toBe(12_000);
    expect(monitorRefetchInterval(undefined, { response: { status: 403 } }, 1)).toBe(false);
    expect(monitorRefetchInterval(undefined, { response: { status: 401 } }, 1)).toBe(false);
  });

  it('uses the server cadence on success and bounded backoff on transient errors', () => {
    expect(monitorRefetchInterval({ sampleIntervalMs: 9_000 } as never, undefined, 0)).toBe(9_000);
    expect(monitorRefetchInterval(undefined, new Error('temporary'), 1)).toBe(5_000);
    expect(monitorRefetchInterval(undefined, new Error('temporary'), 99)).toBe(60_000);
  });
});
