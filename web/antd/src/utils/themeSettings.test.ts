import defaultSettings from '../../config/defaultSettings';
import { applyAuthenticatedThemeProfiles, THEME_AUTH_SESSION_KEY } from './themeSession';
import {
  buildLayoutSettings,
  applyThemeProfiles,
  clearUserThemeProfile,
  compareThemeRevisions,
  CODE_THEME_DEFAULTS,
  normalizeThemeOverrides,
  getVerifiedThemeAuthSessionId,
  markThemeScopeDegraded,
  parseThemeScopeResource,
  reconcileThemeScopeResource,
  resolveThemeSettings,
} from './themeSettings';

describe('theme settings precedence', () => {
  const storageValues = new Map<string, string>();

  beforeEach(() => {
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
    localStorage.setItem(THEME_AUTH_SESSION_KEY, 'session-a');
  });

  it('keeps the seven-field code contract complete and excludes deployment settings', () => {
    expect(CODE_THEME_DEFAULTS).toEqual({
      navTheme: 'realDark',
      colorPrimary: '#1890ff',
      layout: 'mix',
      contentWidth: 'Fluid',
      fixedHeader: false,
      fixSiderbar: true,
      colorWeak: false,
    });
    expect(CODE_THEME_DEFAULTS).not.toHaveProperty('pwa');
    expect(CODE_THEME_DEFAULTS).not.toHaveProperty('splitMenus');
  });

  it('normalizes legacy boolean strings without losing false', () => {
    expect(
      normalizeThemeOverrides({
        fixedHeader: 'false',
        fixSiderbar: 'true',
        colorWeak: false,
      }),
    ).toEqual({
      fixedHeader: false,
      fixSiderbar: true,
      colorWeak: false,
    });
  });

  it('resolves user over application over immutable code defaults', () => {
    const code = { ...CODE_THEME_DEFAULTS };
    const application = { navTheme: 'light', fixSiderbar: false, fixedHeader: true };
    const user = { navTheme: 'realDark', fixedHeader: 'false' };

    expect(resolveThemeSettings(code, application, user)).toMatchObject({
      navTheme: 'realDark',
      fixSiderbar: false,
      fixedHeader: false,
    });
    expect(code).toEqual(CODE_THEME_DEFAULTS);
    expect(Object.isFrozen(CODE_THEME_DEFAULTS)).toBe(true);
  });

  it('ignores deleted, empty, and invalid overrides', () => {
    expect(
      normalizeThemeOverrides({
        navTheme: '',
        layout: 'unknown',
        colorPrimary: null,
        fixedHeader: null,
        pwa: true,
      }),
    ).toEqual({});
  });

  it('accepts only six-digit colors and normalizes them to lower case', () => {
    expect(normalizeThemeOverrides({ colorPrimary: '#A1B2C3' })).toEqual({
      colorPrimary: '#a1b2c3',
    });
    expect(normalizeThemeOverrides({ colorPrimary: '#abc' })).toEqual({});
    expect(normalizeThemeOverrides({ colorPrimary: '#a1b2c3ff' })).toEqual({});
  });

  it('builds fresh layout settings without mutating the imported defaults', () => {
    const snapshot = { ...defaultSettings };
    const first = buildLayoutSettings(
      { base: { websiteName: 'Tenant' }, theme: { fixSiderbar: false } },
      { theme: { fixedHeader: 'true' } },
    );
    const second = buildLayoutSettings();

    expect(first).toMatchObject({ title: 'Tenant', fixSiderbar: false, fixedHeader: true });
    expect(second).toMatchObject(CODE_THEME_DEFAULTS);
    expect(first).toMatchObject({
      pwa: defaultSettings.pwa,
      iconfontUrl: defaultSettings.iconfontUrl,
      token: defaultSettings.token,
    });
    expect(defaultSettings).toEqual(snapshot);
    expect(first).not.toBe(second);
  });

  it('replaces a prior user theme at login and clears it at logout', () => {
    const previous = {
      currentUser: { id: 'old' },
      appConfig: { theme: { navTheme: 'light', fixSiderbar: false } },
      userConfig: { theme: { navTheme: 'realDark', fixedHeader: 'true' } },
      settings: buildLayoutSettings(
        { theme: { navTheme: 'light', fixSiderbar: false } },
        { theme: { navTheme: 'realDark', fixedHeader: 'true' } },
      ),
    };

    const next = applyAuthenticatedThemeProfiles(
      previous,
      { id: 'new' },
      {
        userConfig: { theme: { fixedHeader: 'false' } },
      },
      'session-a',
    );
    expect(next.settings).toMatchObject({
      navTheme: 'light',
      fixSiderbar: false,
      fixedHeader: false,
    });

    const loggedOut = clearUserThemeProfile(next);
    expect(loggedOut.userConfig).toBeUndefined();
    expect(loggedOut.settings).toMatchObject({
      navTheme: 'light',
      fixSiderbar: false,
      fixedHeader: CODE_THEME_DEFAULTS.fixedHeader,
    });
  });

  it('marks login profile failures degraded without treating fallbacks as authoritative', () => {
    const base: any = {
      appConfig: { theme: { navTheme: 'light' } },
      userConfig: { theme: { navTheme: 'realDark' } },
    };
    const missingApplication = applyAuthenticatedThemeProfiles(
      base,
      { id: 'new-user' },
      { userConfig: { theme: { fixedHeader: true } } },
      'session-a',
    );
    expect(missingApplication.themeRuntime?.degradedScopes).toEqual(['application']);
    expect(missingApplication.settings?.fixedHeader).toBe(true);

    const missingUser = applyAuthenticatedThemeProfiles(
      base,
      { id: 'new-user' },
      { appConfig: { theme: { navTheme: 'light' } } },
      'session-a',
    );
    expect(missingUser.themeRuntime?.degradedScopes).toEqual(['user']);
    expect(missingUser.userConfig).toBeUndefined();
    expect(missingUser.settings?.navTheme).toBe('light');
  });

  it('orders arbitrarily large decimal revisions without numeric coercion', () => {
    expect(compareThemeRevisions('99999999999999999999', '100000000000000000000')).toBe(-1);
    expect(compareThemeRevisions('00042', '42')).toBe(0);
    expect(() => compareThemeRevisions('1e3', '1000')).toThrow();
  });

  it('parses compatible legacy resources and strict versioned metadata', () => {
    expect(parseThemeScopeResource({ fixedHeader: 'false' }, 'user')).toEqual({
      scope: 'user',
      revision: '0',
      overrides: { fixedHeader: false },
      versioned: false,
    });
    expect(
      parseThemeScopeResource(
        {
          navTheme: 'light',
          _meta: { v: 1, scope: 'application', revision: '0007' },
        },
        'application',
      ),
    ).toMatchObject({ revision: '7', overrides: { navTheme: 'light' }, versioned: true });
    expect(() =>
      parseThemeScopeResource({ _meta: { v: 1, scope: 'user', revision: '2' } }, 'application'),
    ).toThrow();
  });

  it('reconciles only newer resources while preserving personal precedence', () => {
    const initial = applyAuthenticatedThemeProfiles(
      {},
      { id: 'user-a' },
      {
        appConfig: {
          theme: {
            navTheme: 'light',
            _meta: { v: 1, scope: 'application', revision: '4' },
          },
        },
        userConfig: {
          theme: {
            navTheme: 'realDark',
            _meta: { v: 1, scope: 'user', revision: '8' },
          },
        },
      },
      'session-a',
    );
    const stale = reconcileThemeScopeResource(initial, {
      scope: 'application',
      revision: '3',
      overrides: { navTheme: 'realDark' },
      versioned: true,
    });
    expect(stale.status).toBe('stale');

    const next = reconcileThemeScopeResource(initial, {
      scope: 'application',
      revision: '5',
      overrides: { navTheme: 'light', fixedHeader: true },
      versioned: true,
    });
    expect(next.status).toBe('applied');
    expect((next.state as any).settings).toMatchObject({
      navTheme: 'realDark',
      fixedHeader: true,
    });
    expect((next.state as any).themeRuntime?.layers.application?.revision).toBe('5');
  });

  it('keeps a newer application revision when an older login batch applies a new user', () => {
    const runtime = applyThemeProfiles<any>(
      {
        currentUser: { id: 'old-user' },
        themeRuntime: {
          schemaVersion: 1 as const,
          authSessionId: 'old-session',
          layers: {},
        },
      },
      {
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '10' },
        },
      },
      {
        theme: {
          navTheme: 'realDark',
          _meta: { v: 1, scope: 'user', revision: '8' },
        },
      },
    );

    const next = applyAuthenticatedThemeProfiles(
      runtime,
      { id: 'new-user' },
      {
        appConfig: {
          theme: {
            fixedHeader: false,
            _meta: { v: 1, scope: 'application', revision: '9' },
          },
        },
        userConfig: {
          theme: {
            colorWeak: true,
            _meta: { v: 1, scope: 'user', revision: '1' },
          },
        },
      },
      'session-a',
    );

    expect(next.currentUser).toEqual({ id: 'new-user' });
    expect(next.themeRuntime?.layers.application?.revision).toBe('10');
    expect(next.themeRuntime?.layers.user?.revision).toBe('1');
    expect(next.settings).toMatchObject({ fixedHeader: true, colorWeak: true });
  });

  it('lets an authoritative read replace an untrusted degraded snapshot revision', () => {
    const hinted = markThemeScopeDegraded<any>(
      applyThemeProfiles<any>(
        {},
        {
          theme: {
            fixedHeader: true,
            _meta: {
              v: 1,
              scope: 'application',
              revision: '99999999999999999999',
            },
          },
        },
        undefined,
      ),
      'application',
    );

    const reconciled = reconcileThemeScopeResource(
      hinted,
      {
        scope: 'application',
        revision: '7',
        overrides: { fixedHeader: false },
        versioned: true,
      },
      { authoritative: true },
    );

    expect(reconciled.status).toBe('applied');
    expect(reconciled.state.themeRuntime?.layers.application?.revision).toBe('7');
    expect(reconciled.state.settings?.fixedHeader).toBe(false);
    expect(reconciled.state.themeRuntime?.degradedScopes).toBeUndefined();
  });

  it('flags divergent payloads at the same authoritative revision', () => {
    const state = applyThemeProfiles(
      {},
      {
        theme: {
          fixedHeader: false,
          _meta: { v: 1, scope: 'application', revision: '4' },
        },
      },
      undefined,
    );
    const result = reconcileThemeScopeResource(state, {
      scope: 'application',
      revision: '4',
      overrides: { fixedHeader: true },
      versioned: true,
    });
    expect(result.status).toBe('same-revision-conflict');

    const authoritative = reconcileThemeScopeResource(
      markThemeScopeDegraded(state, 'application'),
      {
        scope: 'application',
        revision: '4',
        overrides: { fixedHeader: true },
        versioned: true,
      },
      { authoritative: true },
    );
    expect(authoritative.status).toBe('applied');
    expect((authoritative.state as any).settings.fixedHeader).toBe(true);
    expect((authoritative.state as any).themeRuntime.degradedScopes).toBeUndefined();
  });

  it('never rebinds a verified runtime identity from shared storage alone', () => {
    const state = {
      themeRuntime: {
        schemaVersion: 1 as const,
        layers: {},
        authSessionId: 'session-a',
      },
    };
    expect(getVerifiedThemeAuthSessionId(state, 'session-a')).toBe('session-a');
    expect(getVerifiedThemeAuthSessionId(state, 'session-b')).toBeUndefined();
    expect(getVerifiedThemeAuthSessionId(undefined, 'session-b')).toBeUndefined();
  });

  it('marks unavailable scopes degraded even when no snapshot layer exists', () => {
    const applicationUnavailable = markThemeScopeDegraded<any>({}, 'application');
    const allUnavailable = markThemeScopeDegraded(applicationUnavailable, 'user');

    expect(allUnavailable.themeRuntime?.layers).toEqual({});
    expect(allUnavailable.themeRuntime?.degradedScopes).toEqual(['application', 'user']);
  });
});
