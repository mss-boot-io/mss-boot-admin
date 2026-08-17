import {
  areThemeResourcesEqual,
  compareThemeRevisions,
  normalizeThemeOverrides,
  normalizeThemeRevision,
  type ThemeOverrides,
  type ThemeScope,
  type ThemeScopeResource,
} from './contract';

export const THEME_SYNC_CHANNEL = 'mss.antd-v6.theme.v1';
export const THEME_SYNC_STORAGE_KEY = 'mss.antd-v6.theme.event.v1';
export const THEME_SYNC_EVENT_TTL_MS = 5 * 60 * 1000;

interface ThemeSyncEventBase {
  v: 1;
  id: string;
  origin: string;
  issuedAt: number;
}

export interface ThemeScopeUpdatedEvent extends ThemeSyncEventBase {
  type: 'scope-updated';
  scope: ThemeScope;
  revision: string;
  overrides: ThemeOverrides;
  authSessionId?: string;
}

export interface ThemeIdentityClearedEvent extends ThemeSyncEventBase {
  type: 'identity-cleared';
  previousAuthSessionId: string;
}

export type ThemeSyncEvent = ThemeScopeUpdatedEvent | ThemeIdentityClearedEvent;
export type ThemeSyncListener = (event: ThemeSyncEvent) => void;

const listeners = new Set<ThemeSyncListener>();
const seenEventIDs = new Set<string>();
const seenEventQueue: string[] = [];
let sequence = 0;
let channel: BroadcastChannel | undefined;
let transportStarted = false;

function randomID(): string {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID();
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = ((bytes[6] ?? 0) & 0x0f) | 0x40;
  bytes[8] = ((bytes[8] ?? 0) & 0x3f) | 0x80;
  const hex = [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

const tabID = randomID();

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function rememberEvent(id: string): boolean {
  if (seenEventIDs.has(id)) return false;
  seenEventIDs.add(id);
  seenEventQueue.push(id);
  if (seenEventQueue.length > 256) {
    const oldest = seenEventQueue.shift();
    if (oldest) seenEventIDs.delete(oldest);
  }
  return true;
}

export function parseThemeSyncEvent(value: unknown, now = Date.now()): ThemeSyncEvent | undefined {
  if (!isRecord(value) || value.v !== 1 || typeof value.id !== 'string' || !value.id) {
    return undefined;
  }
  if (typeof value.origin !== 'string' || !value.origin || typeof value.issuedAt !== 'number') {
    return undefined;
  }
  if (
    !Number.isFinite(value.issuedAt) ||
    Math.abs(now - value.issuedAt) > THEME_SYNC_EVENT_TTL_MS
  ) {
    return undefined;
  }

  if (value.type === 'identity-cleared') {
    if (typeof value.previousAuthSessionId !== 'string' || !value.previousAuthSessionId) {
      return undefined;
    }
    return {
      v: 1,
      id: value.id,
      origin: value.origin,
      issuedAt: value.issuedAt,
      type: 'identity-cleared',
      previousAuthSessionId: value.previousAuthSessionId,
    };
  }
  if (value.type !== 'scope-updated') return undefined;
  if (value.scope !== 'application' && value.scope !== 'user') return undefined;
  const revision = normalizeThemeRevision(value.revision);
  if (revision === undefined) return undefined;
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

export type ThemeEventDecision = 'apply' | 'duplicate' | 'stale' | 'conflict' | 'wrong-session';

export function decideThemeScopeEvent(
  event: ThemeScopeUpdatedEvent,
  current: ThemeScopeResource | undefined,
  authSessionId?: string,
): ThemeEventDecision {
  if (event.scope === 'user' && event.authSessionId !== authSessionId) return 'wrong-session';
  if (!current) return 'apply';
  const incoming: ThemeScopeResource = {
    scope: event.scope,
    revision: event.revision,
    overrides: event.overrides,
  };
  const order = compareThemeRevisions(incoming.revision, current.revision);
  if (order < 0) return 'stale';
  if (order > 0) return 'apply';
  return areThemeResourcesEqual(incoming, current) ? 'duplicate' : 'conflict';
}

export function isThemeSyncEventFromCurrentDocument(event: ThemeSyncEvent): boolean {
  return event.origin === tabID;
}

function dispatch(value: unknown) {
  const event = parseThemeSyncEvent(value);
  if (!event || !rememberEvent(event.id)) return;
  for (const listener of listeners) listener(event);
}

function onStorage(event: StorageEvent) {
  if (event.key !== THEME_SYNC_STORAGE_KEY || !event.newValue) return;
  try {
    dispatch(JSON.parse(event.newValue));
  } catch {
    // Invalid browser data is untrusted and ignored.
  }
}

function startTransport() {
  if (transportStarted || typeof window === 'undefined') return;
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
        // Continue with the storage fallback.
      }
      channel = undefined;
    }
  }
  try {
    window.localStorage.setItem(THEME_SYNC_STORAGE_KEY, JSON.stringify(event));
    window.localStorage.removeItem(THEME_SYNC_STORAGE_KEY);
  } catch {
    // The local subscriber has already received the event.
  }
}

function eventBase(): ThemeSyncEventBase {
  sequence += 1;
  return { v: 1, id: `${tabID}:${sequence}`, origin: tabID, issuedAt: Date.now() };
}

export function publishThemeScopeResource(
  resource: ThemeScopeResource,
  authSessionId?: string,
): ThemeScopeUpdatedEvent | undefined {
  if (resource.scope === 'user' && !authSessionId) return undefined;
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

export function publishThemeIdentityCleared(
  previousAuthSessionId: string,
): ThemeIdentityClearedEvent | undefined {
  if (!previousAuthSessionId) return undefined;
  const event: ThemeIdentityClearedEvent = {
    ...eventBase(),
    type: 'identity-cleared',
    previousAuthSessionId,
  };
  dispatch(event);
  publishExternal(event);
  return event;
}

export function subscribeThemeSync(listener: ThemeSyncListener): () => void {
  startTransport();
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function closeThemeSyncTransport() {
  if (typeof window !== 'undefined' && transportStarted) {
    window.removeEventListener('storage', onStorage);
  }
  try {
    channel?.close();
  } catch {
    // Cleanup is best-effort.
  }
  channel = undefined;
  transportStarted = false;
  seenEventIDs.clear();
  seenEventQueue.splice(0, seenEventQueue.length);
}
