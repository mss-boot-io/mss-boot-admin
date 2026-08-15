import { describe, expect, it } from 'vitest';
import {
  CODE_THEME_DEFAULTS,
  compareThemeRevisions,
  isOAuthProviderEnabled,
  normalizeThemeOverrides,
  parseApplicationProfile,
  parseThemeScopeResource,
  resolveTheme,
} from './contract';

describe('v6 layered theme contract', () => {
  it('resolves every field independently as code < application < user', () => {
    const application = parseThemeScopeResource(
      { fixedHeader: true, colorWeak: true, colorPrimary: '#ABCDEF' },
      'application',
    );
    const user = parseThemeScopeResource({ colorWeak: false, layout: 'side' }, 'user');
    const resolved = resolveTheme(application, user);

    expect(resolved.settings).toEqual({
      ...CODE_THEME_DEFAULTS,
      fixedHeader: true,
      colorWeak: false,
      colorPrimary: '#abcdef',
      layout: 'side',
    });
    expect(resolved.sources.fixedHeader).toBe('application');
    expect(resolved.sources.colorWeak).toBe('user');
    expect(resolved.sources.navTheme).toBe('code');
  });

  it('normalizes legacy booleans and ignores unknown or invalid values', () => {
    expect(
      normalizeThemeOverrides({
        fixedHeader: 'false',
        fixSiderbar: 'true',
        navTheme: 'dark',
        colorPrimary: 'red',
        pwa: true,
      }),
    ).toEqual({ fixedHeader: false, fixSiderbar: true });
  });

  it('requires valid canonical metadata and compares unbounded decimal revisions', () => {
    expect(
      parseThemeScopeResource(
        { layout: 'mix', _meta: { v: 1, scope: 'user', revision: '0007' } },
        'user',
      ),
    ).toMatchObject({ revision: '7', versioned: true });
    expect(() =>
      parseThemeScopeResource({ _meta: { v: 1, scope: 'application', revision: 7 } }, 'user'),
    ).toThrow('Invalid user theme resource metadata');
    expect(compareThemeRevisions('100000000000000000000', '99999999999999999999')).toBe(1);
  });

  it('reads only the allowlisted theme layer from the public profile', () => {
    const profile = parseApplicationProfile({
      base: { websiteName: 'MSS' },
      security: { registerEnabled: true },
      theme: {
        navTheme: 'realDark',
        pwa: true,
        _meta: { v: 1, scope: 'application', revision: '4' },
      },
      private: { secret: 'must not be consumed' },
    });
    expect(profile.theme.overrides).toEqual({ navTheme: 'realDark' });
    expect(profile).not.toHaveProperty('private');
  });

  it('enables OAuth providers only from an explicit public application flag', () => {
    const profile = parseApplicationProfile({
      security: { githubEnabled: true, larkEnabled: 'false', unknownEnabled: true },
    });
    expect(isOAuthProviderEnabled(profile, 'github')).toBe(true);
    expect(isOAuthProviderEnabled(profile, 'lark')).toBe(false);
    expect(isOAuthProviderEnabled(undefined, 'github')).toBe(false);
  });
});
