import {
  areThemeResourcesEqual,
  compareThemeRevisions,
  parseThemeScopeResource,
  serializeThemeScopeResource,
  type ThemeScope,
  type ThemeScopeResource,
} from './contract';
import {
  APPLICATION_THEME_SNAPSHOT_KEY,
  THEME_AUTH_SESSION_KEY,
  THEME_SNAPSHOT_TTL_MS,
  USER_THEME_SNAPSHOT_PREFIX,
} from './storage';

export {
  APPLICATION_THEME_SNAPSHOT_KEY,
  THEME_AUTH_SESSION_KEY,
  THEME_SNAPSHOT_TTL_MS,
  USER_THEME_SNAPSHOT_PREFIX,
} from './storage';

const THEME_SNAPSHOT_LOCK_PREFIX = 'mss.antd-v6.theme.snapshot.lock.v1:';

interface ThemeSnapshotEnvelope {
  v: 1;
  expiresAt: number;
  resource: Record<string, unknown>;
}

interface ThemeAuthBinding {
  v: 1;
  id: string;
  subject: string;
}

interface ThemeSnapshotWriteOptions {
  authoritativePrevious?: ThemeScopeResource;
}

function browserStorage(): Storage | undefined {
  try {
    return typeof window === 'undefined' ? undefined : window.localStorage;
  } catch {
    return undefined;
  }
}

function randomSessionID(): string {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID();
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = ((bytes[6] ?? 0) & 0x0f) | 0x40;
  bytes[8] = ((bytes[8] ?? 0) & 0x3f) | 0x80;
  const hex = [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function userSnapshotKey(authSessionId: string) {
  return `${USER_THEME_SNAPSHOT_PREFIX}${authSessionId}`;
}

function snapshotKey(scope: ThemeScope, authSessionId?: string) {
  if (scope === 'application') return APPLICATION_THEME_SNAPSHOT_KEY;
  if (!authSessionId) throw new Error('Personal theme snapshots require an auth session');
  return userSnapshotKey(authSessionId);
}

function browserLockManager(): LockManager | undefined {
  try {
    return typeof navigator !== 'undefined' && typeof navigator.locks?.request === 'function'
      ? navigator.locks
      : undefined;
  } catch {
    return undefined;
  }
}

function removeUserSnapshot(authSessionId: string) {
  const storage = browserStorage();
  const key = userSnapshotKey(authSessionId);
  try {
    storage?.removeItem(key);
  } catch {
    // Continue with locked cleanup when available.
  }
  const locks = browserLockManager();
  if (!storage || !locks) return;
  void locks
    .request(`${THEME_SNAPSHOT_LOCK_PREFIX}${key}`, () => {
      try {
        storage.removeItem(key);
      } catch {
        // Disabled storage means there is no reusable snapshot.
      }
    })
    .catch(() => {});
}

function broadcastIdentityCleared(previousAuthSessionId: string) {
  void import('./sync')
    .then(({ publishThemeIdentityCleared }) => publishThemeIdentityCleared(previousAuthSessionId))
    .catch(() => {
      // Local cleanup is complete even when the convergence transport cannot
      // be loaded in this document.
    });
}

export function getThemeAuthSessionId(
  storage: Pick<Storage, 'getItem'> | undefined = browserStorage(),
): string | undefined {
  try {
    const raw = storage?.getItem(THEME_AUTH_SESSION_KEY);
    if (!raw) return undefined;
    const value = JSON.parse(raw) as Partial<ThemeAuthBinding>;
    return value.v === 1 && typeof value.id === 'string' && value.id ? value.id : undefined;
  } catch {
    return undefined;
  }
}

export function getVerifiedThemeAuthSessionId(
  subject: string | undefined,
  storage: Pick<Storage, 'getItem'> | undefined = browserStorage(),
): string | undefined {
  if (!subject) return undefined;
  try {
    const raw = storage?.getItem(THEME_AUTH_SESSION_KEY);
    if (!raw) return undefined;
    const value = JSON.parse(raw) as Partial<ThemeAuthBinding>;
    return value.v === 1 && value.subject === subject && typeof value.id === 'string' && value.id
      ? value.id
      : undefined;
  } catch {
    return undefined;
  }
}

export function rotateThemeAuthSession(subject: string): string {
  const storage = browserStorage();
  const previous = getThemeAuthSessionId(storage);
  const next = randomSessionID();
  try {
    storage?.setItem(
      THEME_AUTH_SESSION_KEY,
      JSON.stringify({ v: 1, id: next, subject } satisfies ThemeAuthBinding),
    );
  } catch {
    // Without storage, personal snapshots and events stay disabled.
  }
  if (previous) {
    removeUserSnapshot(previous);
    // Publish only after the new binding is visible so another tab that
    // reboots immediately cannot reattach the prior identity snapshot.
    broadcastIdentityCleared(previous);
  }
  return next;
}

export function ensureThemeAuthSession(subject: string): string {
  return getVerifiedThemeAuthSessionId(subject) ?? rotateThemeAuthSession(subject);
}

export function clearThemeIdentitySession(
  options: { broadcast?: boolean; expectedSessionId?: string } = {},
): string | undefined {
  const storage = browserStorage();
  const current = getThemeAuthSessionId(storage);
  const previous = options.expectedSessionId ?? current;
  if (previous) removeUserSnapshot(previous);
  try {
    if (!options.expectedSessionId || current === options.expectedSessionId) {
      storage?.removeItem(THEME_AUTH_SESSION_KEY);
    }
  } catch {
    // Memory/runtime cleanup does not depend on browser persistence.
  }
  if (previous && options.broadcast !== false) broadcastIdentityCleared(previous);
  return previous;
}

export function readThemeSnapshot(
  scope: ThemeScope,
  authSessionId?: string,
  now = Date.now(),
): ThemeScopeResource | undefined {
  if (scope === 'user' && !authSessionId) return undefined;
  const storage = browserStorage();
  if (!storage) return undefined;
  const key = snapshotKey(scope, authSessionId);
  try {
    const raw = storage.getItem(key);
    if (!raw) return undefined;
    const envelope = JSON.parse(raw) as Partial<ThemeSnapshotEnvelope>;
    if (
      envelope.v !== 1 ||
      typeof envelope.expiresAt !== 'number' ||
      !Number.isFinite(envelope.expiresAt) ||
      envelope.expiresAt <= now ||
      envelope.expiresAt > now + THEME_SNAPSHOT_TTL_MS ||
      !envelope.resource
    ) {
      storage.removeItem(key);
      return undefined;
    }
    const resource = parseThemeScopeResource(envelope.resource, scope);
    if (!resource.versioned) {
      storage.removeItem(key);
      return undefined;
    }
    return resource;
  } catch {
    try {
      storage.removeItem(key);
    } catch {
      // Invalid state is ignored when storage cannot be cleaned.
    }
    return undefined;
  }
}

export async function writeThemeSnapshot(
  resource: ThemeScopeResource,
  authSessionId?: string,
  now = Date.now(),
  options: ThemeSnapshotWriteOptions = {},
): Promise<boolean> {
  if (!resource.versioned || (resource.scope === 'user' && !authSessionId)) return false;
  const storage = browserStorage();
  const locks = browserLockManager();
  // localStorage has no compare-and-set. Without Web Locks, declining the
  // optimization is safer than allowing a delayed tab to overwrite new state.
  if (!storage || !locks) return false;
  const key = snapshotKey(resource.scope, authSessionId);
  try {
    return await locks.request(`${THEME_SNAPSHOT_LOCK_PREFIX}${key}`, () => {
      if (
        resource.scope === 'user' &&
        (!authSessionId || getThemeAuthSessionId() !== authSessionId)
      ) {
        return false;
      }
      const existing = readThemeSnapshot(resource.scope, authSessionId, now);
      const replacingExactHint = Boolean(
        existing &&
          options.authoritativePrevious &&
          areThemeResourcesEqual(existing, options.authoritativePrevious),
      );
      if (existing && !replacingExactHint) {
        const order = compareThemeRevisions(resource.revision, existing.revision);
        if (order < 0 || (order === 0 && !areThemeResourcesEqual(resource, existing))) {
          return false;
        }
      }
      const envelope: ThemeSnapshotEnvelope = {
        v: 1,
        expiresAt: now + THEME_SNAPSHOT_TTL_MS,
        resource: serializeThemeScopeResource(resource),
      };
      storage.setItem(key, JSON.stringify(envelope));
      return true;
    });
  } catch {
    return false;
  }
}

export interface ThemeBootstrapSnapshot {
  application?: ThemeScopeResource;
}

export function readThemeBootstrapSnapshot(): ThemeBootstrapSnapshot {
  return {
    application: readThemeSnapshot('application', undefined),
  };
}
