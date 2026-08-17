import { beforeEach, describe, expect, it } from 'vitest';
import {
  BROWSER_SESSION_CLEAR_ENDPOINT,
  BROWSER_SESSION_LOGIN_ENDPOINT,
  BROWSER_SESSION_LOGOUT_ENDPOINT,
  BROWSER_SESSION_METADATA_KEY,
  BROWSER_SESSION_REFRESH_ENDPOINT,
  browserSessionRefreshDelay,
  clearBrowserSessionMetadata,
  isPublicPath,
  readBrowserSessionExpiry,
  recordBrowserSessionResponse,
  requireCredentialFreeSessionResponse,
} from './session';

describe('isPublicPath', () => {
  it('keeps only explicit authentication routes public', () => {
    expect(isPublicPath('/user/login')).toBe(true);
    expect(isPublicPath('/user/oauth/callback/github')).toBe(true);
    expect(isPublicPath('/user/callback/github')).toBe(false);
    expect(isPublicPath('/user/oauth/callback/github/extra')).toBe(false);
    expect(isPublicPath('/workplace')).toBe(false);
    expect(isPublicPath('/users')).toBe(false);
  });
});

describe('browser session contract', () => {
  beforeEach(() => window.localStorage.clear());

  it('uses only the dedicated cookie-session endpoints', () => {
    expect(BROWSER_SESSION_LOGIN_ENDPOINT).toBe('/user/session/login');
    expect(BROWSER_SESSION_REFRESH_ENDPOINT).toBe('/user/session/refresh-token');
    expect(BROWSER_SESSION_LOGOUT_ENDPOINT).toBe('/online-sessions/logout');
    expect(BROWSER_SESSION_CLEAR_ENDPOINT).toBe('/user/auth-cookie/clear');
  });

  it('rejects a browser response that exposes a bearer credential', () => {
    expect(
      requireCredentialFreeSessionResponse({ code: 200, expire: '2026-08-17T00:00:00Z' }),
    ).toEqual({
      code: 200,
      expire: '2026-08-17T00:00:00Z',
    });
    expect(() => requireCredentialFreeSessionResponse({ code: 200 })).toThrow(/expiry/);
    expect(() =>
      requireCredentialFreeSessionResponse({ code: 200, token: 'must-not-enter-v6' }),
    ).toThrow(/credential/);
    expect(() =>
      requireCredentialFreeSessionResponse({ accessToken: 'must-not-enter-v6' }),
    ).toThrow(/credential/);
    expect(() =>
      requireCredentialFreeSessionResponse({ refreshToken: 'must-not-enter-v6' }),
    ).toThrow(/credential/);
  });

  it('stores only namespaced non-secret expiry metadata and derives the safety window', () => {
    const now = Date.parse('2026-08-16T00:00:00Z');
    const expiresAt = recordBrowserSessionResponse(
      { code: 200, expire: '2026-08-16T12:00:00Z' },
      now,
    );

    expect(readBrowserSessionExpiry()).toBe(expiresAt);
    expect(browserSessionRefreshDelay(expiresAt, now)).toBe(11 * 60 * 60 * 1000 + 55 * 60 * 1000);
    expect(JSON.parse(window.localStorage.getItem(BROWSER_SESSION_METADATA_KEY) ?? '{}')).toEqual({
      v: 1,
      expiresAt,
    });
    expect(window.localStorage.getItem(BROWSER_SESSION_METADATA_KEY)).not.toContain('token');

    clearBrowserSessionMetadata();
    expect(readBrowserSessionExpiry()).toBeUndefined();
  });
});
