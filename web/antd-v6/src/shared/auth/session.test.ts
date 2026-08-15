import { describe, expect, it } from 'vitest';
import {
  BROWSER_SESSION_CLEAR_ENDPOINT,
  BROWSER_SESSION_LOGIN_ENDPOINT,
  BROWSER_SESSION_LOGOUT_ENDPOINT,
  BROWSER_SESSION_REFRESH_ENDPOINT,
  isPublicPath,
  requireCredentialFreeSessionResponse,
} from './session';

describe('isPublicPath', () => {
  it('keeps only explicit authentication routes public', () => {
    expect(isPublicPath('/user/login')).toBe(true);
    expect(isPublicPath('/user/callback/github')).toBe(true);
    expect(isPublicPath('/user/oauth/callback/github')).toBe(true);
    expect(isPublicPath('/user/callback/github/extra')).toBe(false);
    expect(isPublicPath('/workplace')).toBe(false);
    expect(isPublicPath('/users')).toBe(false);
  });
});

describe('browser session contract', () => {
  it('uses only the dedicated cookie-session endpoints', () => {
    expect(BROWSER_SESSION_LOGIN_ENDPOINT).toBe('/user/session/login');
    expect(BROWSER_SESSION_REFRESH_ENDPOINT).toBe('/user/session/refresh-token');
    expect(BROWSER_SESSION_LOGOUT_ENDPOINT).toBe('/online-sessions/logout');
    expect(BROWSER_SESSION_CLEAR_ENDPOINT).toBe('/user/auth-cookie/clear');
  });

  it('rejects a browser response that exposes a bearer credential', () => {
    expect(requireCredentialFreeSessionResponse({ code: 200, expire: 'tomorrow' })).toEqual({
      code: 200,
      expire: 'tomorrow',
    });
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
});
