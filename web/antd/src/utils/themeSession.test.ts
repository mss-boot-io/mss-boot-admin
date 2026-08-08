import {
  APPLICATION_THEME_SNAPSHOT_KEY,
  applyAuthenticatedThemeProfiles,
  applyThemeDocumentHints,
  applyThemeFirstPaintHint,
  clearThemeIdentitySession,
  getThemeAuthSessionId,
  isThemeBootstrapIdentityActive,
  readThemeBootstrapProfiles,
  readThemeSnapshot,
  rotateThemeAuthSession,
  loadAuthenticatedThemeProfiles,
  THEME_AUTH_SESSION_KEY,
  THEME_PROFILE_LOAD_TIMEOUT_MS,
  THEME_SNAPSHOT_TTL_MS,
  writeThemeSnapshot,
  writeAuthenticatedThemeSnapshots,
} from './themeSession';
import { closeThemeSyncTransport } from './themeSync';
import { getAppConfigsProfile } from '@/services/admin/appConfig';
import { getUserConfigsProfile } from '@/services/admin/userConfig';

jest.mock('@/services/admin/appConfig', () => ({
  getAppConfigsProfile: jest.fn(),
}));

jest.mock('@/services/admin/userConfig', () => ({
  getUserConfigsProfile: jest.fn(),
}));

describe('theme browser snapshots and identity binding', () => {
  const storageValues = new Map<string, string>();
  const originalLocksDescriptor = Object.getOwnPropertyDescriptor(navigator, 'locks');
  let lockRequest: jest.Mock;

  const installSerialLockManager = () => {
    const pendingByName = new Map<string, Promise<unknown>>();
    lockRequest = jest.fn((name: string, callback: () => unknown) => {
      const previous = pendingByName.get(name) || Promise.resolve();
      const next = previous.then(callback);
      pendingByName.set(
        name,
        next.then(
          () => undefined,
          () => undefined,
        ),
      );
      return next;
    });
    Object.defineProperty(navigator, 'locks', {
      configurable: true,
      value: { request: lockRequest },
    });
  };

  const restoreLockManager = () => {
    if (originalLocksDescriptor) {
      Object.defineProperty(navigator, 'locks', originalLocksDescriptor);
    } else {
      Reflect.deleteProperty(navigator, 'locks');
    }
  };

  beforeEach(() => {
    installSerialLockManager();
    storageValues.clear();
    (localStorage.getItem as jest.Mock).mockImplementation(
      (key: string) => storageValues.get(key) ?? null,
    );
    (localStorage.setItem as jest.Mock).mockImplementation((key: string, value: string) => {
      storageValues.set(key, String(value));
    });
    (localStorage.removeItem as jest.Mock).mockImplementation((key: string) => {
      storageValues.delete(key);
    });
    clearThemeIdentitySession({ broadcast: false });
    document.documentElement.removeAttribute('data-mss-theme');
    document.documentElement.style.removeProperty('color-scheme');
    document.documentElement.style.removeProperty('--mss-theme-color-primary');
    jest.clearAllMocks();
  });

  afterEach(() => {
    jest.useRealTimers();
    clearThemeIdentitySession({ broadcast: false });
    closeThemeSyncTransport();
    restoreLockManager();
  });

  it('stores only a versioned canonical application theme and rejects expired data', async () => {
    const resource = {
      scope: 'application' as const,
      revision: '12',
      overrides: { navTheme: 'light' as const, colorPrimary: '#aabbcc' },
      versioned: true,
    };
    expect(await writeThemeSnapshot(resource, undefined, 100)).toBe(true);
    expect(readThemeSnapshot('application', undefined, 101)).toEqual(resource);

    const raw = JSON.parse(localStorage.getItem(APPLICATION_THEME_SNAPSHOT_KEY) || '{}');
    expect(raw.resource).toEqual({
      navTheme: 'light',
      colorPrimary: '#aabbcc',
      _meta: { v: 1, scope: 'application', revision: '12' },
    });
    expect(raw.resource).not.toHaveProperty('pwa');
    expect(readThemeSnapshot('application', undefined, 100 + THEME_SNAPSHOT_TTL_MS + 1)).toBe(
      undefined,
    );
  });

  it('serializes snapshot writes and lets authority replace only the exact warm hint', async () => {
    const warmHint = {
      scope: 'application' as const,
      revision: '12',
      overrides: { navTheme: 'light' as const },
      versioned: true,
    };
    expect(await writeThemeSnapshot(warmHint, undefined, 100)).toBe(true);
    expect(
      await writeThemeSnapshot(
        {
          scope: 'application',
          revision: '11',
          overrides: { navTheme: 'realDark' },
          versioned: true,
        },
        undefined,
        101,
      ),
    ).toBe(false);
    expect(
      await writeThemeSnapshot(
        {
          ...warmHint,
          overrides: { navTheme: 'realDark' },
        },
        undefined,
        102,
      ),
    ).toBe(false);
    expect(readThemeSnapshot('application', undefined, 103)).toEqual(warmHint);

    const authoritative = {
      scope: 'application' as const,
      revision: '10',
      overrides: { navTheme: 'realDark' as const },
      versioned: true,
    };
    expect(
      await writeThemeSnapshot(authoritative, undefined, 104, {
        authoritativePrevious: warmHint,
      }),
    ).toBe(true);
    expect(readThemeSnapshot('application', undefined, 105)).toEqual(authoritative);

    const newerRuntime = {
      ...warmHint,
      revision: '11',
      overrides: { navTheme: 'light' as const, colorWeak: true },
    };
    const [newerWritten, staleAuthorityWritten] = await Promise.all([
      writeThemeSnapshot(newerRuntime, undefined, 106),
      writeThemeSnapshot(authoritative, undefined, 107, {
        authoritativePrevious: warmHint,
      }),
    ]);
    expect(newerWritten).toBe(true);
    expect(staleAuthorityWritten).toBe(false);
    expect(readThemeSnapshot('application', undefined, 108)).toEqual(newerRuntime);
    expect(new Set(lockRequest.mock.calls.map(([name]) => name)).size).toBe(1);
  });

  it('does not persist snapshots when Web Locks are unavailable', async () => {
    Object.defineProperty(navigator, 'locks', {
      configurable: true,
      value: undefined,
    });

    expect(
      await writeThemeSnapshot({
        scope: 'application',
        revision: '1',
        overrides: { fixedHeader: true },
        versioned: true,
      }),
    ).toBe(false);
    expect(localStorage.setItem).not.toHaveBeenCalled();
  });

  it('rejects snapshots whose expiry is beyond the one-day trust window', () => {
    localStorage.setItem(
      APPLICATION_THEME_SNAPSHOT_KEY,
      JSON.stringify({
        v: 1,
        expiresAt: 100 + THEME_SNAPSHOT_TTL_MS + 1,
        resource: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '99999999999999999999' },
        },
      }),
    );

    expect(readThemeSnapshot('application', undefined, 100)).toBeUndefined();
    expect(localStorage.getItem(APPLICATION_THEME_SNAPSHOT_KEY)).toBeNull();
  });

  it('warms an anonymous application theme without creating an auth session', async () => {
    await writeThemeSnapshot({
      scope: 'application',
      revision: '2',
      overrides: { navTheme: 'light' },
      versioned: true,
    });
    jest.clearAllMocks();

    const bootstrap = readThemeBootstrapProfiles();

    expect(bootstrap.application?.revision).toBe('2');
    expect(bootstrap.user).toBeUndefined();
    expect(bootstrap.authSessionId).toBeUndefined();
    expect(localStorage.setItem).not.toHaveBeenCalledWith(
      THEME_AUTH_SESSION_KEY,
      expect.any(String),
    );
  });

  it('binds a personal snapshot to a random auth session and clears it on rotation', async () => {
    const firstSession = rotateThemeAuthSession({ persistent: true });
    const resource = {
      scope: 'user' as const,
      revision: '3',
      overrides: { fixedHeader: true },
      versioned: true,
    };
    expect(await writeThemeSnapshot(resource, firstSession)).toBe(true);
    expect(readThemeSnapshot('user', firstSession)).toEqual(resource);

    const secondSession = rotateThemeAuthSession({ persistent: true });
    expect(secondSession).not.toBe(firstSession);
    expect(getThemeAuthSessionId()).toBe(secondSession);
    expect(localStorage.getItem(THEME_AUTH_SESSION_KEY)).toBe(secondSession);
    expect(readThemeSnapshot('user', firstSession)).toBeUndefined();
  });

  it('uses a cryptographically secure UUID when randomUUID is not exposed', () => {
    const randomUUIDDescriptor = Object.getOwnPropertyDescriptor(crypto, 'randomUUID');
    Object.defineProperty(crypto, 'randomUUID', {
      configurable: true,
      value: undefined,
    });

    try {
      const session = rotateThemeAuthSession({ persistent: true });
      expect(session).toMatch(
        /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
      );
    } finally {
      if (randomUUIDDescriptor) {
        Object.defineProperty(crypto, 'randomUUID', randomUUIDDescriptor);
      } else {
        Reflect.deleteProperty(crypto, 'randomUUID');
      }
    }
  });

  it('rejects a delayed login response after another tab rotates the identity session', async () => {
    const sessionA = rotateThemeAuthSession({ persistent: true });
    const state = {
      currentUser: { id: 'previous' },
      themeRuntime: { schemaVersion: 1 as const, authSessionId: sessionA, layers: {} },
    };
    const profiles = {
      appConfig: {
        theme: { fixedHeader: true, _meta: { v: 1, scope: 'application', revision: '8' } },
      },
      userConfig: {
        theme: { colorWeak: true, _meta: { v: 1, scope: 'user', revision: '9' } },
      },
    };

    const sessionB = rotateThemeAuthSession({ persistent: true });
    const next = applyAuthenticatedThemeProfiles(state, { id: 'user-a' }, profiles, sessionA);

    expect(sessionB).not.toBe(sessionA);
    expect(next).toBe(state);
    expect(await writeAuthenticatedThemeSnapshots(state, sessionA)).toBe(false);
    expect(readThemeSnapshot('user', sessionB)).toBeUndefined();
  });

  it('fails closed when identity rotates while an authenticated snapshot lock is pending', async () => {
    const sessionA = rotateThemeAuthSession({ persistent: true });
    let releaseApplicationLock!: () => void;
    let applicationLockStarted!: () => void;
    const lockStarted = new Promise<void>((resolve) => {
      applicationLockStarted = resolve;
    });
    const releaseLock = new Promise<void>((resolve) => {
      releaseApplicationLock = resolve;
    });
    lockRequest.mockImplementationOnce(async (_name: string, callback: () => unknown) => {
      applicationLockStarted();
      await releaseLock;
      return callback();
    });
    const state = {
      currentUser: { id: 'user-a' },
      themeRuntime: {
        schemaVersion: 1 as const,
        authSessionId: sessionA,
        layers: {
          application: {
            scope: 'application' as const,
            revision: '8',
            overrides: { fixedHeader: true },
            versioned: true,
          },
          user: {
            scope: 'user' as const,
            revision: '9',
            overrides: { colorWeak: true },
            versioned: true,
          },
        },
      },
    };

    const writing = writeAuthenticatedThemeSnapshots(state, sessionA);
    await lockStarted;
    const sessionB = rotateThemeAuthSession({ persistent: true });
    releaseApplicationLock();

    expect(sessionB).not.toBe(sessionA);
    await expect(writing).resolves.toBe(false);
    expect(readThemeSnapshot('user', sessionA)).toBeUndefined();
    expect(readThemeSnapshot('user', sessionB)).toBeUndefined();
  });

  it('does not resurrect an old personal snapshot after identity rotation while its lock waits', async () => {
    const sessionA = rotateThemeAuthSession({ persistent: true });
    let releaseUserLock!: () => void;
    let userLockStarted!: () => void;
    const lockStarted = new Promise<void>((resolve) => {
      userLockStarted = resolve;
    });
    const releaseLock = new Promise<void>((resolve) => {
      releaseUserLock = resolve;
    });
    lockRequest.mockImplementationOnce(async (_name: string, callback: () => unknown) => {
      userLockStarted();
      await releaseLock;
      return callback();
    });

    const writing = writeThemeSnapshot(
      {
        scope: 'user',
        revision: '9',
        overrides: { colorWeak: true },
        versioned: true,
      },
      sessionA,
    );
    await lockStarted;
    const sessionB = rotateThemeAuthSession({ persistent: true });
    releaseUserLock();

    expect(sessionB).not.toBe(sessionA);
    await expect(writing).resolves.toBe(false);
    expect(readThemeSnapshot('user', sessionA)).toBeUndefined();
    expect(readThemeSnapshot('user', sessionB)).toBeUndefined();
  });

  it('skips degraded authenticated layers instead of reviving stale higher revisions', async () => {
    const authSessionId = rotateThemeAuthSession({ persistent: true });
    const authoritative = {
      scope: 'application' as const,
      revision: '10',
      overrides: { fixedHeader: false },
      versioned: true,
    };
    await writeThemeSnapshot(authoritative);
    const state = {
      currentUser: { id: 'user-a' },
      themeRuntime: {
        schemaVersion: 1 as const,
        authSessionId,
        layers: {
          application: {
            scope: 'application' as const,
            revision: '12',
            overrides: { fixedHeader: true },
            versioned: true,
          },
        },
        degradedScopes: ['application' as const],
      },
    };

    await expect(writeAuthenticatedThemeSnapshots(state, authSessionId)).resolves.toBe(true);
    expect(readThemeSnapshot('application')).toEqual(authoritative);
  });

  it('writes the final reconciled layers instead of an older login profile batch', async () => {
    const authSessionId = rotateThemeAuthSession({ persistent: true });
    const application = {
      scope: 'application' as const,
      revision: '11',
      overrides: { fixedHeader: true },
      versioned: true,
    };
    const state = {
      currentUser: { id: 'previous' },
      appConfig: {
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '11' },
        },
      },
      themeRuntime: {
        schemaVersion: 1 as const,
        authSessionId,
        layers: { application },
      },
    };
    await writeThemeSnapshot(application);

    const next = applyAuthenticatedThemeProfiles(
      state,
      { id: 'current' },
      {
        appConfig: {
          theme: {
            fixedHeader: false,
            _meta: { v: 1, scope: 'application', revision: '10' },
          },
        },
        userConfig: {
          theme: {
            colorWeak: true,
            _meta: { v: 1, scope: 'user', revision: '2' },
          },
        },
      },
      authSessionId,
    );

    expect(next.themeRuntime.layers.application).toEqual(application);
    expect(await writeAuthenticatedThemeSnapshots(next, authSessionId)).toBe(true);
    expect(readThemeSnapshot('application')?.revision).toBe('11');
    expect(readThemeSnapshot('user', authSessionId)).toMatchObject({
      revision: '2',
      overrides: { colorWeak: true },
    });
  });

  it('invalidates a delayed bootstrap result when token and session rotate together', async () => {
    localStorage.setItem('token', 'token-a');
    const sessionA = rotateThemeAuthSession({ persistent: true });
    let release!: () => void;
    const delayed = new Promise<void>((resolve) => {
      release = resolve;
    }).then(() =>
      isThemeBootstrapIdentityActive('token-a', sessionA, localStorage.getItem('token')),
    );

    localStorage.setItem('token', 'token-b');
    const sessionB = rotateThemeAuthSession({ persistent: true });
    release();

    expect(sessionB).not.toBe(sessionA);
    await expect(delayed).resolves.toBe(false);
  });

  it('does not clear another tab persistent identity when an OAuth session ends', () => {
    const transientSession = rotateThemeAuthSession({ persistent: false });
    localStorage.setItem(THEME_AUTH_SESSION_KEY, 'persistent-session-b');

    expect(clearThemeIdentitySession({ broadcast: false })).toBe(transientSession);
    expect(localStorage.getItem(THEME_AUTH_SESSION_KEY)).toBe('persistent-session-b');
  });

  it('does not fail rendering when the storage backend rejects reads', () => {
    const inaccessibleStorage = {
      getItem: () => {
        throw new Error('storage access denied');
      },
    };

    expect(getThemeAuthSessionId(inaccessibleStorage)).toBeUndefined();
    const transientSession = rotateThemeAuthSession({ persistent: false });
    expect(getThemeAuthSessionId(inaccessibleStorage)).toBe(transientSession);
  });

  it('applies a synchronous warm-load color-scheme hint without other profile data', async () => {
    await writeThemeSnapshot({
      scope: 'application',
      revision: '4',
      overrides: { navTheme: 'light', colorPrimary: '#abcdef' },
      versioned: true,
    });

    applyThemeFirstPaintHint();

    expect(document.documentElement.dataset.mssTheme).toBe('light');
    expect(document.documentElement.style.colorScheme).toBe('light');
    expect(document.documentElement.style.getPropertyValue('--mss-theme-color-primary')).toBe(
      '#abcdef',
    );
  });

  it('updates document hints when the effective runtime theme changes', () => {
    applyThemeDocumentHints({ navTheme: 'realDark', colorPrimary: '#112233' });
    expect(document.documentElement.dataset.mssTheme).toBe('realDark');
    expect(document.documentElement.style.colorScheme).toBe('dark');

    applyThemeDocumentHints({ navTheme: 'light', colorPrimary: '#aabbcc' });
    expect(document.documentElement.dataset.mssTheme).toBe('light');
    expect(document.documentElement.style.colorScheme).toBe('light');
    expect(document.documentElement.style.getPropertyValue('--mss-theme-color-primary')).toBe(
      '#aabbcc',
    );
  });

  it('bounds authenticated theme profile loading when one endpoint never settles', async () => {
    jest.useFakeTimers();
    (getAppConfigsProfile as jest.Mock).mockReturnValue(new Promise(() => {}));
    (getUserConfigsProfile as jest.Mock).mockResolvedValue({
      theme: { _meta: { v: 1, scope: 'user', revision: '7' } },
    });

    const loading = loadAuthenticatedThemeProfiles();
    await jest.advanceTimersByTimeAsync(THEME_PROFILE_LOAD_TIMEOUT_MS);

    await expect(loading).resolves.toEqual({
      appConfig: undefined,
      userConfig: { theme: { _meta: { v: 1, scope: 'user', revision: '7' } } },
    });
  });
});
