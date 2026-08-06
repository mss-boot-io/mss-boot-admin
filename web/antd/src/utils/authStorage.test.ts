import { clearAuthStorage, clearNonPersistentAuthStorage } from './authStorage';

describe('Admin authentication storage lifecycle', () => {
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

    expect(clearNonPersistentAuthStorage(storage)).toBe(false);
    expect(storage.removeItem).not.toHaveBeenCalled();
  });

  it('clears every Admin auth key during explicit logout', () => {
    const storage = { removeItem: jest.fn() };
    clearAuthStorage(storage);
    expect(storage.removeItem.mock.calls.map(([key]) => key)).toEqual([
      'token',
      'token.expire',
      'autoLogin',
    ]);
  });
});
