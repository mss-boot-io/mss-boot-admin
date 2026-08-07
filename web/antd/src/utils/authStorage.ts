const AUTH_STORAGE_KEYS = ['token', 'token.expire', 'autoLogin'] as const;

type AuthStorage = Pick<Storage, 'getItem' | 'removeItem'>;

let transientAuthToken: string | undefined;

// OAuth login results remain scoped to the current document. This keeps the
// bearer out of Web Storage while preserving the existing storage contract for
// password, email, phone, and refresh-token login paths.
export function setTransientAuthToken(value: string) {
  if (!value) {
    throw new Error('Admin auth token is required');
  }
  transientAuthToken = value;
}

export function clearTransientAuthToken() {
  transientAuthToken = undefined;
}

export function getAuthToken(storage: Pick<Storage, 'getItem'> = window.localStorage) {
  return transientAuthToken || storage.getItem('token');
}

export function clearAuthStorage(storage: Pick<Storage, 'removeItem'> = window.localStorage) {
  clearTransientAuthToken();
  AUTH_STORAGE_KEYS.forEach((key) => storage.removeItem(key));
}

export function clearNonPersistentAuthStorage(storage: AuthStorage = window.localStorage) {
  clearTransientAuthToken();
  if (storage.getItem('autoLogin') === 'true') {
    return false;
  }
  clearAuthStorage(storage);
  return true;
}
