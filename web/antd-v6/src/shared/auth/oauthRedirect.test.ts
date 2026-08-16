import { beforeEach, describe, expect, it } from 'vitest';
import {
  consumeOAuthLoginRedirect,
  OAUTH_LOGIN_REDIRECT_PREFIX,
  rememberOAuthLoginRedirect,
} from './oauthRedirect';

describe('attempt-bound OAuth login redirects', () => {
  const now = Date.parse('2026-08-16T00:00:00Z');
  const origin = 'https://admin.example.test';

  beforeEach(() => window.sessionStorage.clear());

  it('preserves and consumes a safe deep link only for the matching attempt', () => {
    rememberOAuthLoginRedirect(
      'attempt-1',
      '2026-08-16T00:05:00Z',
      '/users?page=2#details',
      now,
      window.sessionStorage,
      origin,
    );

    expect(
      consumeOAuthLoginRedirect('attempt-2', now, window.sessionStorage, origin),
    ).toBeUndefined();
    expect(consumeOAuthLoginRedirect('attempt-1', now, window.sessionStorage, origin)).toBe(
      '/users?page=2#details',
    );
    expect(
      consumeOAuthLoginRedirect('attempt-1', now, window.sessionStorage, origin),
    ).toBeUndefined();
  });

  it('stores a same-origin fallback and rejects expired or malformed browser data', () => {
    expect(
      rememberOAuthLoginRedirect(
        'attempt-safe',
        '2026-08-16T00:05:00Z',
        'https://evil.example/path',
        now,
        window.sessionStorage,
        origin,
      ),
    ).toBe('/workplace');
    expect(consumeOAuthLoginRedirect('attempt-safe', now, window.sessionStorage, origin)).toBe(
      '/workplace',
    );

    window.sessionStorage.setItem(
      `${OAUTH_LOGIN_REDIRECT_PREFIX}attempt-expired`,
      JSON.stringify({ v: 1, expiresAt: now - 1, redirect: '/users' }),
    );
    expect(
      consumeOAuthLoginRedirect('attempt-expired', now, window.sessionStorage, origin),
    ).toBeUndefined();
    expect(() =>
      rememberOAuthLoginRedirect(
        '../invalid',
        '2026-08-16T00:05:00Z',
        '/users',
        now,
        window.sessionStorage,
        origin,
      ),
    ).toThrow(/invalid/);
  });
});
