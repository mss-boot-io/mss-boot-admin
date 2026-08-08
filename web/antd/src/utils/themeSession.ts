import { getAppConfigsProfile } from '@/services/admin/appConfig';
import { getUserConfigsProfile } from '@/services/admin/userConfig';
import {
  areThemeOverridesEqual,
  buildLayoutSettings,
  clearUserThemeProfile,
  compareThemeRevisions,
  markThemeScopeDegraded,
  parseThemeScopeResource,
  reconcileThemeProfile,
  serializeThemeScopeResource,
  setThemeScopeResource,
  type ConfigProfile,
  type ThemeRuntimeState,
  type ThemeScopeResource,
  type ThemeSettings,
  type ThemeSettingsScope,
} from './themeSettings';
import { publishThemeIdentityCleared } from './themeSync';

export const THEME_AUTH_SESSION_KEY = 'mss.auth.theme-session.v1';
export const APPLICATION_THEME_SNAPSHOT_KEY = 'mss.theme.application.v1';
export const USER_THEME_SNAPSHOT_PREFIX = 'mss.theme.user.v1:';
export const THEME_SNAPSHOT_TTL_MS = 24 * 60 * 60 * 1000;

type ThemeSnapshotEnvelope = {
  v: 1;
  expiresAt: number;
  resource: Record<string, unknown>;
};

type ThemeSnapshotWriteOptions = {
  /**
   * The exact warm-start hint replaced by an authoritative server response.
   * Downgrading that hint is allowed only while holding the cross-document
   * snapshot lock and storage still contains this exact resource.
   */
  authoritativePrevious?: ThemeScopeResource;
};

type AuthenticatedThemeSnapshotWriteOptions = {
  authoritativePrevious?: Partial<Record<ThemeSettingsScope, ThemeScopeResource>>;
};

const THEME_SNAPSHOT_LOCK_PREFIX = 'mss.theme.snapshot.lock.v1:';

let transientThemeAuthSessionId: string | undefined;

const createSessionID = () => {
  try {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
  } catch {
    // Fall through to a non-security identifier; it never authenticates a user.
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
};

function browserStorage(): Storage | undefined {
  try {
    return typeof window === 'undefined' ? undefined : window.localStorage;
  } catch {
    return undefined;
  }
}

function userSnapshotKey(authSessionId: string) {
  return `${USER_THEME_SNAPSHOT_PREFIX}${authSessionId}`;
}

function themeSnapshotKey(scope: ThemeSettingsScope, authSessionId?: string) {
  return scope === 'application'
    ? APPLICATION_THEME_SNAPSHOT_KEY
    : userSnapshotKey(authSessionId!);
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

function themeSnapshotLockName(key: string) {
  return `${THEME_SNAPSHOT_LOCK_PREFIX}${key}`;
}

function removeUserThemeSnapshot(storage: Storage | undefined, authSessionId: string) {
  const key = userSnapshotKey(authSessionId);
  try {
    storage?.removeItem(key);
  } catch {
    // The locked cleanup below is best-effort as well.
  }
  const locks = browserLockManager();
  if (!storage || !locks) return;
  // Rotation itself stays synchronous for the authentication flow, while the
  // same per-key lock guarantees that a previously queued writer cannot
  // resurrect the old identity snapshot after this cleanup request.
  void locks
    .request(themeSnapshotLockName(key), () => {
      try {
        storage.removeItem(key);
      } catch {
        // A disabled storage backend already implies no usable snapshot.
      }
    })
    .catch(() => {});
}

function areThemeResourcesEqual(left: ThemeScopeResource, right: ThemeScopeResource) {
  return (
    left.scope === right.scope &&
    left.revision === right.revision &&
    left.versioned === right.versioned &&
    areThemeOverridesEqual(left.overrides, right.overrides)
  );
}

export function getThemeAuthSessionId(
  storage: Pick<Storage, 'getItem'> | undefined = browserStorage(),
) {
  if (transientThemeAuthSessionId) return transientThemeAuthSessionId;
  try {
    return storage?.getItem(THEME_AUTH_SESSION_KEY) || undefined;
  } catch {
    return undefined;
  }
}

export function rotateThemeAuthSession(options: { persistent: boolean }) {
  const storage = browserStorage();
  const previous = getThemeAuthSessionId(storage);
  if (previous) {
    removeUserThemeSnapshot(storage, previous);
    publishThemeIdentityCleared(previous);
  }

  const next = createSessionID();
  transientThemeAuthSessionId = options.persistent ? undefined : next;
  try {
    if (options.persistent) {
      storage?.setItem(THEME_AUTH_SESSION_KEY, next);
    } else {
      storage?.removeItem(THEME_AUTH_SESSION_KEY);
    }
  } catch {
    if (options.persistent) {
      transientThemeAuthSessionId = next;
    }
  }
  return next;
}

export function ensureThemeAuthSession(options: { persistent: boolean }) {
  return getThemeAuthSessionId() || rotateThemeAuthSession(options);
}

export function clearThemeIdentitySession(options: { broadcast?: boolean } = {}) {
  const storage = browserStorage();
  const transientPrevious = transientThemeAuthSessionId;
  let persistentPrevious: string | undefined;
  try {
    persistentPrevious = storage?.getItem(THEME_AUTH_SESSION_KEY) || undefined;
  } catch {
    persistentPrevious = undefined;
  }
  const previous = transientPrevious || persistentPrevious;
  transientThemeAuthSessionId = undefined;
  try {
    if (previous) removeUserThemeSnapshot(storage, previous);
    // A document-scoped OAuth session must not erase a newer persistent
    // identity established by another tab.
    if (!transientPrevious) storage?.removeItem(THEME_AUTH_SESSION_KEY);
  } catch {
    // Continue clearing memory even when storage is unavailable.
  }
  if (previous && options.broadcast !== false) {
    publishThemeIdentityCleared(previous);
  }
  return previous;
}

export function readThemeSnapshot(
  scope: 'application' | 'user',
  authSessionId = getThemeAuthSessionId(),
  now = Date.now(),
): ThemeScopeResource | undefined {
  if (scope === 'user' && !authSessionId) return undefined;
  const storage = browserStorage();
  if (!storage) return undefined;
  const key = themeSnapshotKey(scope, authSessionId);
  try {
    const raw = storage.getItem(key);
    if (!raw) return undefined;
    const envelope = JSON.parse(raw) as Partial<ThemeSnapshotEnvelope>;
    if (
      envelope.v !== 1 ||
      typeof envelope.expiresAt !== 'number' ||
      !Number.isFinite(envelope.expiresAt) ||
      envelope.expiresAt <= now ||
      envelope.expiresAt > now + THEME_SNAPSHOT_TTL_MS
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
      // Ignore cleanup failures.
    }
    return undefined;
  }
}

export async function writeThemeSnapshot(
  resource: ThemeScopeResource,
  authSessionId = getThemeAuthSessionId(),
  now?: number,
  options: ThemeSnapshotWriteOptions = {},
): Promise<boolean> {
  if (!resource.versioned || (resource.scope === 'user' && !authSessionId)) {
    return false;
  }
  const storage = browserStorage();
  if (!storage) return false;
  const locks = browserLockManager();
  // localStorage has no atomic compare-and-set primitive. Without Web Locks,
  // even a higher-revision read/compare/write can overwrite a still-newer tab
  // between its read and write, so the only safe fallback is no persistence.
  if (!locks) return false;

  const key = themeSnapshotKey(resource.scope, authSessionId);
  try {
    return await locks.request(themeSnapshotLockName(key), () => {
      if (
        resource.scope === 'user' &&
        (!authSessionId || getThemeAuthSessionId() !== authSessionId)
      ) {
        return false;
      }
      const writeTime = now ?? Date.now();
      const existing = readThemeSnapshot(resource.scope, authSessionId, writeTime);
      const replacesAuthoritativeHint = Boolean(
        existing &&
          options.authoritativePrevious &&
          areThemeResourcesEqual(existing, options.authoritativePrevious),
      );
      if (existing && !replacesAuthoritativeHint) {
        const comparison = compareThemeRevisions(resource.revision, existing.revision);
        if (
          comparison < 0 ||
          (comparison === 0 && !areThemeOverridesEqual(resource.overrides, existing.overrides))
        ) {
          return false;
        }
      }
      const envelope: ThemeSnapshotEnvelope = {
        v: 1,
        expiresAt: writeTime + THEME_SNAPSHOT_TTL_MS,
        resource: serializeThemeScopeResource(resource),
      };
      storage.setItem(key, JSON.stringify(envelope));
      return true;
    });
  } catch {
    return false;
  }
}

export function profileFromThemeResource(resource?: ThemeScopeResource): ConfigProfile | undefined {
  return resource ? setThemeScopeResource(undefined, resource) : undefined;
}

export function readThemeBootstrapProfiles() {
  const authSessionId = getThemeAuthSessionId();
  const application = readThemeSnapshot('application', authSessionId);
  const user = readThemeSnapshot('user', authSessionId);
  return {
    authSessionId,
    application,
    user,
    appConfig: profileFromThemeResource(application),
    userConfig: profileFromThemeResource(user),
  };
}

export function applyThemeDocumentHints(
  settings: Pick<ThemeSettings, 'navTheme' | 'colorPrimary'>,
) {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  root.dataset.mssTheme = settings.navTheme;
  root.style.colorScheme = settings.navTheme === 'realDark' ? 'dark' : 'light';
  root.style.setProperty('--mss-theme-color-primary', settings.colorPrimary);
}

export function applyThemeFirstPaintHint() {
  if (typeof document === 'undefined') return;
  const cached = readThemeBootstrapProfiles();
  const settings = buildLayoutSettings(cached.appConfig, cached.userConfig);
  applyThemeDocumentHints(settings);
}

export function isThemeAuthSessionActive(
  expectedAuthSessionId: string | undefined,
  activeAuthSessionId = getThemeAuthSessionId(),
) {
  return Boolean(
    expectedAuthSessionId &&
      activeAuthSessionId &&
      expectedAuthSessionId === activeAuthSessionId,
  );
}

export function isThemeBootstrapIdentityActive(
  expectedToken: string,
  expectedAuthSessionId: string,
  activeToken: string | null | undefined,
) {
  return expectedToken === activeToken && isThemeAuthSessionActive(expectedAuthSessionId);
}

export async function writeAuthenticatedThemeSnapshots(
  state: ThemeRuntimeState,
  authSessionId = getThemeAuthSessionId(),
  options: AuthenticatedThemeSnapshotWriteOptions = {},
): Promise<boolean> {
  if (!isThemeAuthSessionActive(authSessionId)) {
    return false;
  }
  const application = state.themeRuntime?.layers.application;
  const user = state.themeRuntime?.layers.user;
  const degradedScopes = state.themeRuntime?.degradedScopes || [];
  let applicationWritten = true;
  if (application && !degradedScopes.includes('application')) {
    applicationWritten = await writeThemeSnapshot(application, undefined, undefined, {
      authoritativePrevious: options.authoritativePrevious?.application,
    });
  }
  if (!isThemeAuthSessionActive(authSessionId)) return false;
  let userWritten = true;
  if (user && authSessionId && !degradedScopes.includes('user')) {
    userWritten = await writeThemeSnapshot(user, authSessionId, undefined, {
      authoritativePrevious: options.authoritativePrevious?.user,
    });
  }
  return (
    applicationWritten && userWritten && isThemeAuthSessionActive(authSessionId)
  );
}

export type AuthenticatedThemeProfiles = {
  appConfig?: unknown;
  userConfig?: unknown;
};

export const THEME_PROFILE_LOAD_TIMEOUT_MS = 4000;

function loadThemeProfileWithTimeout<T>(request: () => Promise<T>, timeoutMs: number) {
  return new Promise<T | undefined>((resolve) => {
    let settled = false;
    let timer: ReturnType<typeof setTimeout>;
    const finish = (value: T | undefined) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      resolve(value);
    };
    timer = setTimeout(() => finish(undefined), timeoutMs);
    Promise.resolve()
      .then(request)
      .then((value) => finish(value))
      .catch(() => finish(undefined));
  });
}

export async function loadAuthenticatedThemeProfiles(
  timeoutMs = THEME_PROFILE_LOAD_TIMEOUT_MS,
): Promise<AuthenticatedThemeProfiles> {
  const [appConfig, userConfig] = await Promise.all([
    loadThemeProfileWithTimeout(
      () => getAppConfigsProfile({ skipErrorHandler: true }),
      timeoutMs,
    ),
    loadThemeProfileWithTimeout(
      () => getUserConfigsProfile({ skipErrorHandler: true }),
      timeoutMs,
    ),
  ]);
  return {
    appConfig,
    userConfig,
  };
}

export function applyAuthenticatedThemeProfiles<
  T extends ThemeRuntimeState & { currentUser?: unknown },
>(
  state: T,
  currentUser: unknown,
  profiles: AuthenticatedThemeProfiles,
  authSessionId: string,
): T {
  if (!isThemeAuthSessionActive(authSessionId)) {
    return state;
  }
  // A new identity never inherits the previous user's layer. Application
  // resources remain monotonic if a faster runtime reconciliation already
  // applied a newer revision than this login batch carries.
  let next = clearUserThemeProfile(state);
  next = {
    ...next,
    currentUser,
    themeRuntime: {
      schemaVersion: 1,
      layers: next.themeRuntime?.layers || {},
      ...next.themeRuntime,
      authSessionId,
    },
  };
  if (profiles.appConfig !== undefined) {
    next = reconcileThemeProfile(next, profiles.appConfig, 'application', {
      allowLegacyReplace: true,
      authoritative: true,
      authSessionId,
    }).state;
  } else {
    next = markThemeScopeDegraded(next, 'application');
  }
  if (profiles.userConfig !== undefined) {
    next = reconcileThemeProfile(next, profiles.userConfig, 'user', {
      allowLegacyReplace: true,
      authoritative: true,
      authSessionId,
    }).state;
  } else {
    next = markThemeScopeDegraded(next, 'user');
  }
  return {
    ...next,
    currentUser,
    themeRuntime: {
      schemaVersion: 1,
      layers: next.themeRuntime?.layers || {},
      ...next.themeRuntime,
      authSessionId,
    },
  };
}
