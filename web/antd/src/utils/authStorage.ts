const AUTH_STORAGE_KEYS = ['token', 'token.expire', 'autoLogin'] as const;

type AuthStorage = Pick<Storage, 'getItem' | 'removeItem'>;

export function clearAuthStorage(storage: Pick<Storage, 'removeItem'> = window.localStorage) {
  AUTH_STORAGE_KEYS.forEach((key) => storage.removeItem(key));
}

export function clearNonPersistentAuthStorage(
  storage: AuthStorage = window.localStorage,
) {
  if (storage.getItem('autoLogin') === 'true') {
    return false;
  }
  clearAuthStorage(storage);
  return true;
}
