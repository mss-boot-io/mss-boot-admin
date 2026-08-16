export const BROWSER_SESSION_METADATA_KEY = 'mss.antd-v6.auth.session.v1';
export const BROWSER_SESSION_METADATA_EVENT = 'mss:antd-v6:auth-session';
export const BROWSER_SESSION_REFRESH_LOCK = 'mss.antd-v6.auth.session-refresh.v1';
export const BROWSER_SESSION_REFRESH_SAFETY_MS = 5 * 60 * 1000;
export const BROWSER_SESSION_REFRESH_RETRY_MS = 30 * 1000;

export interface BrowserSessionResponse {
  code?: number;
  expire: string;
}

interface BrowserSessionMetadata {
  v: 1;
  expiresAt: number;
}

function browserStorage(): Storage | undefined {
  try {
    return typeof window === 'undefined' ? undefined : window.localStorage;
  } catch {
    return undefined;
  }
}

function notifyBrowserSessionMetadataChanged(): void {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event(BROWSER_SESSION_METADATA_EVENT));
  }
}

export function recordBrowserSessionResponse(
  response: BrowserSessionResponse,
  now = Date.now(),
  storage = browserStorage(),
): number {
  const expiresAt = Date.parse(response.expire);
  if (!Number.isFinite(expiresAt) || expiresAt <= now) {
    throw new Error('Browser session expiry must be in the future');
  }
  const metadata: BrowserSessionMetadata = { v: 1, expiresAt };
  try {
    storage?.setItem(BROWSER_SESSION_METADATA_KEY, JSON.stringify(metadata));
  } catch {
    // The HttpOnly cookie remains authoritative when optional metadata storage is unavailable.
  }
  notifyBrowserSessionMetadataChanged();
  return expiresAt;
}

export function readBrowserSessionExpiry(storage = browserStorage()): number | undefined {
  try {
    const raw = storage?.getItem(BROWSER_SESSION_METADATA_KEY);
    if (!raw) return undefined;
    const metadata = JSON.parse(raw) as Partial<BrowserSessionMetadata>;
    if (
      metadata.v !== 1 ||
      typeof metadata.expiresAt !== 'number' ||
      !Number.isFinite(metadata.expiresAt)
    ) {
      storage?.removeItem(BROWSER_SESSION_METADATA_KEY);
      return undefined;
    }
    return metadata.expiresAt;
  } catch {
    return undefined;
  }
}

export function clearBrowserSessionMetadata(storage = browserStorage()): void {
  try {
    storage?.removeItem(BROWSER_SESSION_METADATA_KEY);
  } catch {
    // Clearing an optional, non-secret scheduling hint is best-effort.
  }
  notifyBrowserSessionMetadataChanged();
}

export function browserSessionRefreshDelay(expiresAt: number, now = Date.now()): number {
  return Math.max(0, expiresAt - now - BROWSER_SESSION_REFRESH_SAFETY_MS);
}
