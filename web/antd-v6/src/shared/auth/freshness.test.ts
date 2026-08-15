import { describe, expect, it, vi } from 'vitest';
import {
  AUTHORIZATION_REFRESH_EVENT,
  authorizationStateSignature,
  isAuthorizationBootstrapQuery,
  requestAuthorizationRefresh,
  shouldRefreshAuthorization,
} from './freshness';
import type { CurrentUser } from './types';

function user(permissions: Record<string, boolean>): CurrentUser {
  return {
    id: 'user-1',
    roleID: 'role-1',
    role: { id: 'role-1', root: false, status: 'enabled' },
    permissions,
  };
}

describe('authorization freshness contract', () => {
  it('uses a stable privilege signature and detects executable menu changes', () => {
    const first = authorizationStateSignature(user({ '/b': true, '/a': true }), [
      { key: 'one', path: '/one', permission: '/a' },
    ]);
    const reorderedPermissions = authorizationStateSignature(user({ '/a': true, '/b': true }), [
      { key: 'one', path: '/one', permission: '/a' },
    ]);
    const changedMenu = authorizationStateSignature(user({ '/a': true, '/b': true }), [
      { key: 'two', path: '/two', permission: '/b' },
    ]);
    expect(reorderedPermissions).toBe(first);
    expect(changedMenu).not.toBe(first);
  });

  it('retains only exact startup query families during privilege eviction', () => {
    expect(isAuthorizationBootstrapQuery(['identity', 'current-user'])).toBe(true);
    expect(isAuthorizationBootstrapQuery(['authorization', 'menu', 'user-1'])).toBe(true);
    expect(isAuthorizationBootstrapQuery(['configuration', 'theme', 'user'])).toBe(true);
    expect(isAuthorizationBootstrapQuery(['account', 'user-1', 'access-tokens'])).toBe(false);
    expect(isAuthorizationBootstrapQuery(['configuration', 'system-secrets'])).toBe(false);
  });

  it('throttles passive checks and emits an explicit local event', () => {
    expect(shouldRefreshAuthorization(1_000, 30_999)).toBe(false);
    expect(shouldRefreshAuthorization(1_000, 31_000)).toBe(true);
    const listener = vi.fn();
    window.addEventListener(AUTHORIZATION_REFRESH_EVENT, listener);
    requestAuthorizationRefresh();
    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener(AUTHORIZATION_REFRESH_EVENT, listener);
  });
});
