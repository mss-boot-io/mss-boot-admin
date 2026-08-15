import { describe, expect, it } from 'vitest';
import enUS from './en-US';
import zhCN from './zh-CN';

describe('application locale contract', () => {
  it('keeps English and Chinese message keys synchronized', () => {
    expect(Object.keys(zhCN).sort()).toEqual(Object.keys(enUS).sort());
  });

  it.each([
    'menu.system',
    'menu.system.language',
    'menu.system.option',
    'menu.super-permission',
    'menu.super-permission.app-config',
  ])('defines the dynamic menu key %s', (key) => {
    expect(enUS).toHaveProperty(key);
    expect(zhCN).toHaveProperty(key);
  });
});
