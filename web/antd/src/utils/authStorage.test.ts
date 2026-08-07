import {
  clearAuthStorage,
  clearNonPersistentAuthStorage,
  clearTransientAuthToken,
  getAuthToken,
  setTransientAuthToken,
} from './authStorage';

describe('Admin authentication storage lifecycle', () => {
  beforeEach(() => {
    clearTransientAuthToken();
  });

  it('prefers a document-scoped OAuth token without writing Web Storage', () => {
    const storage = { getItem: jest.fn(() => 'persisted-admin-token') };

    setTransientAuthToken('oauth-admin-token');

    expect(getAuthToken(storage)).toBe('oauth-admin-token');
    expect(storage.getItem).not.toHaveBeenCalled();
  });

  it('falls back to the persisted token after the transient session is cleared', () => {
    const storage = { getItem: jest.fn(() => 'persisted-admin-token') };

    setTransientAuthToken('oauth-admin-token');
    clearTransientAuthToken();

    expect(getAuthToken(storage)).toBe('persisted-admin-token');
    expect(storage.getItem).toHaveBeenCalledWith('token');
  });

  it('clears a stale non-persistent login before rendering the login page', () => {
    const values: Record<string, string> = {
      token: 'stale-admin-token',
      'token.expire': 'expired',
      autoLogin: 'false',
    };
    const storage = {
      getItem: jest.fn((key: string) => values[key] || null),
      removeItem: jest.fn((key: string) => delete values[key]),
    };

    expect(clearNonPersistentAuthStorage(storage)).toBe(true);
    expect(storage.removeItem.mock.calls.map(([key]) => key)).toEqual([
      'token',
      'token.expire',
      'autoLogin',
    ]);
    expect(values).toEqual({});
  });

  it('preserves an explicitly remembered login for refresh', () => {
    const storage = {
      getItem: jest.fn(() => 'true'),
      removeItem: jest.fn(),
    };

    setTransientAuthToken('oauth-admin-token');

    expect(clearNonPersistentAuthStorage(storage)).toBe(false);
    expect(storage.removeItem).not.toHaveBeenCalled();
    expect(getAuthToken({ getItem: jest.fn(() => null) })).toBeNull();
  });

  it('clears every Admin auth key during explicit logout', () => {
    const storage = { removeItem: jest.fn() };
    setTransientAuthToken('oauth-admin-token');

    clearAuthStorage(storage);

    expect(storage.removeItem.mock.calls.map(([key]) => key)).toEqual([
      'token',
      'token.expire',
      'autoLogin',
    ]);
    expect(getAuthToken({ getItem: jest.fn(() => null) })).toBeNull();
  });
});
