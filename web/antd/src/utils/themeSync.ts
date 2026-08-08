import {
  areThemeOverridesEqual,
  compareThemeRevisions,
  normalizeThemeOverrides,
  normalizeThemeRevision,
  type ThemeOverrides,
  type ThemeRevision,
  type ThemeScopeResource,
  type ThemeSettingsScope,
} from './themeSettings';

export const THEME_SYNC_CHANNEL = 'mss.theme.v1';
export const THEME_SYNC_STORAGE_KEY = 'mss.theme.event.v1';
export const THEME_SYNC_EVENT_TTL_MS = 5 * 60 * 1000;

type ThemeSyncEventBase = {
  v: 1;
  id: string;
  origin: string;
  issuedAt: number;
};

export type ThemeScopeUpdatedEvent = ThemeSyncEventBase & {
  type: 'scope-updated';
  scope: ThemeSettingsScope;
  revision: ThemeRevision;
  overrides: ThemeOverrides;
  authSessionId?: string;
};

export type ThemeIdentityClearedEvent = ThemeSyncEventBase & {
  type: 'identity-cleared';
  previousAuthSessionId: string;
};

export type ThemeSyncEvent = ThemeScopeUpdatedEvent | ThemeIdentityClearedEvent;
export type ThemeSyncListener = (event: ThemeSyncEvent) => void;

function createID() {
  try {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
  } catch {
    // Fall through to the non-cryptographic, non-security identifier.
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

const listeners = new Set<ThemeSyncListener>();
const seenEventIDs = new Set<string>();
const seenEventQueue: string[] = [];
const tabID = createID();
let sequence = 0;
let channel: BroadcastChannel | undefined;
let transportStarted = false;

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

function rememberEvent(id: string) {
  if (seenEventIDs.has(id)) {
    return false;
  }
  seenEventIDs.add(id);
  seenEventQueue.push(id);
  if (seenEventQueue.length > 256) {
    const oldest = seenEventQueue.shift();
    if (oldest) seenEventIDs.delete(oldest);
  }
  return true;
}

export function parseThemeSyncEvent(
  value: unknown,
  now: number = Date.now(),
): ThemeSyncEvent | undefined {
  if (!isRecord(value) || value.v !== 1 || typeof value.id !== 'string' || !value.id) {
    return undefined;
  }
  if (typeof value.origin !== 'string' || !value.origin || typeof value.issuedAt !== 'number') {
    return undefined;
  }
  if (
    !Number.isFinite(value.issuedAt) ||
    now - value.issuedAt > THEME_SYNC_EVENT_TTL_MS ||
    value.issuedAt - now > THEME_SYNC_EVENT_TTL_MS
  ) {
    return undefined;
  }

  if (value.type === 'identity-cleared') {
    return typeof value.previousAuthSessionId === 'string' && value.previousAuthSessionId
      ? (value as ThemeIdentityClearedEvent)
      : undefined;
  }
  if (value.type !== 'scope-updated') {
    return undefined;
  }
  if (value.scope !== 'application' && value.scope !== 'user') {
    return undefined;
  }
  const revision = normalizeThemeRevision(value.revision);
  if (revision === undefined) {
    return undefined;
  }
  if (value.scope === 'user' && (typeof value.authSessionId !== 'string' || !value.authSessionId)) {
    return undefined;
  }
  return {
    v: 1,
    id: value.id,
    origin: value.origin,
    issuedAt: value.issuedAt,
    type: 'scope-updated',
    scope: value.scope,
    revision,
    overrides: normalizeThemeOverrides(value.overrides),
    ...(value.scope === 'user' ? { authSessionId: value.authSessionId as string } : {}),
  };
}

export function shouldReconcileThemeScopeEvent(
  event: ThemeScopeUpdatedEvent,
  current: ThemeScopeResource | undefined,
  authSessionId?: string,
) {
  if (event.scope === 'user' && event.authSessionId !== authSessionId) {
    return false;
  }
  if (!current) {
    return true;
  }
  const comparison = compareThemeRevisions(event.revision, current.revision);
  return (
    comparison > 0 ||
    (comparison === 0 && !areThemeOverridesEqual(event.overrides, current.overrides))
  );
}

// Identity events are dispatched locally before they are broadcast. Consumers
// can use this distinction to avoid reloading the document that is already
// performing an explicit login/logout transition while still re-bootstrapping
// other tabs against the newly shared authentication state.
export function isThemeSyncEventFromCurrentDocument(event: ThemeSyncEvent) {
  return event.origin === tabID;
}

function dispatch(value: unknown) {
  const event = parseThemeSyncEvent(value);
  if (!event || !rememberEvent(event.id)) {
    return;
  }
  listeners.forEach((listener) => listener(event));
}

function onStorage(event: StorageEvent) {
  if (event.key !== THEME_SYNC_STORAGE_KEY || !event.newValue) {
    return;
  }
  try {
    dispatch(JSON.parse(event.newValue));
  } catch {
    // Invalid or partially written browser data is ignored.
  }
}

function startTransport() {
  if (transportStarted || typeof window === 'undefined') {
    return;
  }
  transportStarted = true;
  window.addEventListener('storage', onStorage);
  try {
    if (typeof window.BroadcastChannel === 'function') {
      channel = new window.BroadcastChannel(THEME_SYNC_CHANNEL);
      channel.addEventListener('message', (event) => dispatch(event.data));
    }
  } catch {
    channel = undefined;
  }
}

function publishExternal(event: ThemeSyncEvent) {
  startTransport();
  if (channel) {
    try {
      channel.postMessage(event);
      return;
    } catch {
      try {
        channel.close();
      } catch {
        // Ignore cleanup failures and continue with the storage fallback.
      }
      channel = undefined;
    }
  }
  try {
    window.localStorage.setItem(THEME_SYNC_STORAGE_KEY, JSON.stringify(event));
    window.localStorage.removeItem(THEME_SYNC_STORAGE_KEY);
  } catch {
    // Storage may be disabled; the current tab has already applied the event.
  }
}

function eventBase(): ThemeSyncEventBase {
  sequence += 1;
  return {
    v: 1,
    id: `${tabID}:${sequence}`,
    origin: tabID,
    issuedAt: Date.now(),
  };
}

export function publishThemeScopeResource(resource: ThemeScopeResource, authSessionId?: string) {
  if (!resource.versioned || (resource.scope === 'user' && !authSessionId)) {
    return undefined;
  }
  const event: ThemeScopeUpdatedEvent = {
    ...eventBase(),
    type: 'scope-updated',
    scope: resource.scope,
    revision: resource.revision,
    overrides: resource.overrides,
    ...(resource.scope === 'user' ? { authSessionId } : {}),
  };
  dispatch(event);
  publishExternal(event);
  return event;
}

export function publishThemeIdentityCleared(previousAuthSessionId: string) {
  if (!previousAuthSessionId) {
    return undefined;
  }
  const event: ThemeIdentityClearedEvent = {
    ...eventBase(),
    type: 'identity-cleared',
    previousAuthSessionId,
  };
  dispatch(event);
  publishExternal(event);
  return event;
}

export function subscribeThemeSync(listener: ThemeSyncListener) {
  startTransport();
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function closeThemeSyncTransport() {
  if (typeof window !== 'undefined' && transportStarted) {
    window.removeEventListener('storage', onStorage);
  }
  try {
    channel?.close();
  } catch {
    // Ignore cleanup failures.
  }
  channel = undefined;
  transportStarted = false;
  seenEventIDs.clear();
  seenEventQueue.splice(0, seenEventQueue.length);
}
