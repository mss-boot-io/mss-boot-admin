import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { applyThemeFirstPaintHint } from './firstPaint';
import {
  APPLICATION_THEME_SNAPSHOT_KEY,
  clearThemeIdentitySession,
  ensureThemeAuthSession,
  getThemeAuthSessionId,
  getVerifiedThemeAuthSessionId,
  readThemeSnapshot,
  rotateThemeAuthSession,
  THEME_AUTH_SESSION_KEY,
  THEME_SNAPSHOT_TTL_MS,
  writeThemeSnapshot,
} from './snapshot';
import { closeThemeSyncTransport } from './sync';

describe('v6 theme snapshots and identity binding', () => {
  const originalLocks = Object.getOwnPropertyDescriptor(navigator, 'locks');

  beforeEach(() => {
    window.localStorage.clear();
    const queues = new Map<string, Promise<unknown>>();
    Object.defineProperty(navigator, 'locks', {
      configurable: true,
      value: {
        request: (name: string, callback: () => unknown) => {
          const previous = queues.get(name) ?? Promise.resolve();
          const next = previous.then(callback);
          queues.set(
            name,
            next.then(
              () => undefined,
              () => undefined,
            ),
          );
          return next;
        },
      },
    });
  });

  afterEach(() => {
    clearThemeIdentitySession({ broadcast: false });
    closeThemeSyncTransport();
    window.localStorage.clear();
    if (originalLocks) Object.defineProperty(navigator, 'locks', originalLocks);
    else Reflect.deleteProperty(navigator, 'locks');
  });

  it('stores only the canonical public theme and rejects expired snapshots', async () => {
    const resource = {
      scope: 'application' as const,
      revision: '12',
      overrides: { navTheme: 'light' as const, colorPrimary: '#aabbcc' },
    };
    expect(await writeThemeSnapshot(resource, undefined, 100)).toBe(true);
    expect(readThemeSnapshot('application', undefined, 101)).toEqual(resource);
    const stored = JSON.parse(window.localStorage.getItem(APPLICATION_THEME_SNAPSHOT_KEY) ?? '{}');
    expect(stored.resource).toEqual({
      navTheme: 'light',
      colorPrimary: '#aabbcc',
      _meta: { v: 1, scope: 'application', revision: '12' },
    });
    expect(stored.resource).not.toHaveProperty('pwa');
    expect(
      readThemeSnapshot('application', undefined, 100 + THEME_SNAPSHOT_TTL_MS + 1),
    ).toBeUndefined();
  });

  it('serializes monotonic writes and permits only an exact authoritative hint replacement', async () => {
    const hint = {
      scope: 'application' as const,
      revision: '12',
      overrides: { navTheme: 'light' as const },
    };
    await writeThemeSnapshot(hint, undefined, 100);
    const older = {
      scope: 'application' as const,
      revision: '11',
      overrides: { navTheme: 'realDark' as const },
    };
    expect(await writeThemeSnapshot(older, undefined, 101)).toBe(false);
    expect(await writeThemeSnapshot(older, undefined, 102, { authoritativePrevious: hint })).toBe(
      true,
    );
    expect(readThemeSnapshot('application', undefined, 103)).toEqual(older);
  });

  it('declines persistence when Web Locks are unavailable', async () => {
    Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined });
    expect(
      await writeThemeSnapshot({
        scope: 'application',
        revision: '1',
        overrides: { fixedHeader: true },
      }),
    ).toBe(false);
    expect(window.localStorage.getItem(APPLICATION_THEME_SNAPSHOT_KEY)).toBeNull();
  });

  it('binds a personal snapshot to a random session and a verified subject', async () => {
    const first = rotateThemeAuthSession('user-a');
    expect(getThemeAuthSessionId()).toBe(first);
    expect(getVerifiedThemeAuthSessionId('user-a')).toBe(first);
    expect(getVerifiedThemeAuthSessionId('user-b')).toBeUndefined();
    expect(JSON.parse(window.localStorage.getItem(THEME_AUTH_SESSION_KEY) ?? '{}')).toEqual({
      v: 1,
      id: first,
      subject: 'user-a',
    });

    const personal = {
      scope: 'user' as const,
      revision: '3',
      overrides: { fixedHeader: true },
    };
    expect(await writeThemeSnapshot(personal, first)).toBe(true);
    expect(readThemeSnapshot('user', first)).toEqual(personal);

    const second = ensureThemeAuthSession('user-b');
    expect(second).not.toBe(first);
    expect(getVerifiedThemeAuthSessionId('user-b')).toBe(second);
    expect(readThemeSnapshot('user', first)).toBeUndefined();
  });

  it('does not let an old tab clear a newer identity binding', () => {
    const oldSession = rotateThemeAuthSession('user-a');
    const newSession = rotateThemeAuthSession('user-b');
    clearThemeIdentitySession({ broadcast: false, expectedSessionId: oldSession });
    expect(getVerifiedThemeAuthSessionId('user-b')).toBe(newSession);
  });

  it('uses only the public application layer for first paint', async () => {
    await writeThemeSnapshot({
      scope: 'application',
      revision: '2',
      overrides: { navTheme: 'light', colorPrimary: '#112233' },
    });
    const session = rotateThemeAuthSession('user-a');
    await writeThemeSnapshot(
      {
        scope: 'user',
        revision: '4',
        overrides: { navTheme: 'realDark', colorPrimary: '#abcdef' },
      },
      session,
    );

    applyThemeFirstPaintHint();

    expect(document.documentElement.dataset.mssTheme).toBe('light');
    expect(document.documentElement.style.colorScheme).toBe('light');
    expect(document.documentElement.style.getPropertyValue('--mss-theme-color-primary')).toBe(
      '#112233',
    );
  });

  it('ignores valid-looking V5 snapshots and uses only the V6 namespace', () => {
    window.localStorage.setItem(
      'mss.theme.application.v1',
      JSON.stringify({
        v: 1,
        expiresAt: Date.now() + 60_000,
        resource: {
          navTheme: 'realDark',
          colorPrimary: '#1890ff',
          _meta: { v: 1, scope: 'application', revision: '99' },
        },
      }),
    );

    applyThemeFirstPaintHint();

    expect(document.documentElement.dataset.mssTheme).toBe('realDark');
    expect(document.documentElement.style.colorScheme).toBe('dark');
    expect(document.documentElement.style.getPropertyValue('--mss-theme-color-primary')).toBe(
      '#1677ff',
    );
    expect(APPLICATION_THEME_SNAPSHOT_KEY).toBe('mss.antd-v6.theme.application.v1');
    expect(THEME_AUTH_SESSION_KEY).toBe('mss.antd-v6.theme.auth-session.v1');
  });
});
