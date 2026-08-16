import { resolveSafeRedirect } from './redirect';

export const OAUTH_LOGIN_REDIRECT_PREFIX = 'mss.antd-v6.auth.oauth-redirect.v1:';

interface OAuthLoginRedirectEnvelope {
  v: 1;
  expiresAt: number;
  redirect: string;
}

function browserSessionStorage(): Storage | undefined {
  try {
    return typeof window === 'undefined' ? undefined : window.sessionStorage;
  } catch {
    return undefined;
  }
}

function redirectKey(attemptID: string): string | undefined {
  return /^[A-Za-z0-9_-]{1,128}$/.test(attemptID)
    ? `${OAUTH_LOGIN_REDIRECT_PREFIX}${attemptID}`
    : undefined;
}

export function rememberOAuthLoginRedirect(
  attemptID: string,
  expiresAtValue: string,
  requestedRedirect: string | null | undefined,
  now = Date.now(),
  storage = browserSessionStorage(),
  origin = window.location.origin,
): string {
  const key = redirectKey(attemptID);
  const expiresAt = Date.parse(expiresAtValue);
  if (!key || !Number.isFinite(expiresAt) || expiresAt <= now) {
    throw new Error('OAuth login redirect attempt is invalid');
  }
  const redirect = resolveSafeRedirect(requestedRedirect, '/workplace', origin);
  const envelope: OAuthLoginRedirectEnvelope = { v: 1, expiresAt, redirect };
  try {
    storage?.setItem(key, JSON.stringify(envelope));
  } catch {
    // Tab storage is an optional UX hint; the callback safely falls back to the workplace.
  }
  return redirect;
}

export function consumeOAuthLoginRedirect(
  attemptID: string,
  now = Date.now(),
  storage = browserSessionStorage(),
  origin = window.location.origin,
): string | undefined {
  const key = redirectKey(attemptID);
  if (!key) return undefined;
  try {
    const raw = storage?.getItem(key);
    storage?.removeItem(key);
    if (!raw) return undefined;
    const envelope = JSON.parse(raw) as Partial<OAuthLoginRedirectEnvelope>;
    if (
      envelope.v !== 1 ||
      typeof envelope.expiresAt !== 'number' ||
      !Number.isFinite(envelope.expiresAt) ||
      envelope.expiresAt <= now ||
      typeof envelope.redirect !== 'string'
    ) {
      return undefined;
    }
    return resolveSafeRedirect(envelope.redirect, '/workplace', origin);
  } catch {
    return undefined;
  }
}
