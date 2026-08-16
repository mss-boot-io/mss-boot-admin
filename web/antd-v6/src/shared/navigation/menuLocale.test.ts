import { describe, expect, it } from 'vitest';
import { resolveLayoutMenuName, resolveMenuLocaleID } from './menuLocale';

describe('menu locale normalization', () => {
  it.each([
    ['menu.origination.user', 'menu.origination.user'],
    ['origination.user', 'menu.origination.user'],
    ['user', 'menu.user'],
    [' menu.origination.user ', 'menu.origination.user'],
  ])('normalizes %s', (source, expected) => {
    expect(resolveMenuLocaleID(source)).toBe(expected);
  });

  it('recovers the canonical suffix from duplicated migration-era IDs', () => {
    expect(resolveMenuLocaleID('menu.origination.menu.origination.user')).toBe(
      'menu.origination.user',
    );
    expect(
      resolveMenuLocaleID(
        'menu.super-permission.menu.system.appConfig.menu.super-permission.appConfig.control',
      ),
    ).toBe('menu.super-permission.appConfig.control');
  });

  it('returns the name shape expected by ProLayout', () => {
    expect(resolveLayoutMenuName('menu.security.onlineSessions')).toBe('security.onlineSessions');
    expect(resolveLayoutMenuName(undefined)).toBeUndefined();
  });
});
