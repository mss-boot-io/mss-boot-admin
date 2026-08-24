import { describe, expect, it } from 'vitest';
import enUS from './en-US';
import zhCN from './zh-CN';

describe('application locale contract', () => {
  it('keeps English and Chinese message keys synchronized', () => {
    expect(Object.keys(zhCN).sort()).toEqual(Object.keys(enUS).sort());
  });

  it.each([
    'menu.origination',
    'menu.origination.users',
    'menu.origination.departments',
    'menu.origination.posts',
    'menu.authority',
    'menu.authority.role',
    'menu.authority.menu-management',
    'menu.system',
    'menu.system.language',
    'menu.system.option',
    'menu.system.presentation-config',
    'menu.system.presentation-config.draft',
    'menu.system.presentation-config.publish',
    'menu.system.presentation-config.rollback',
    'menu.system.system-log',
    'menu.super-permission',
    'menu.super-permission.app-config',
  ])('defines the dynamic menu key %s', (key) => {
    expect(enUS).toHaveProperty(key);
    expect(zhCN).toHaveProperty(key);
  });
});
