import { describe, expect, it, vi } from 'vitest';
import { registerSupportedLanguageProfile } from './runtime';

describe('language runtime registration', () => {
  it('registers supported overlays with static Ant Design locale data', () => {
    const register = vi.fn();
    registerSupportedLanguageProfile(
      {
        'en-US': { 'menu.language': 'Languages' },
        'zh-CN': { 'menu.language': '语言管理' },
      },
      register,
    );

    expect(register).toHaveBeenCalledTimes(2);
    expect(register).toHaveBeenNthCalledWith(
      1,
      'en-US',
      { 'menu.language': 'Languages' },
      expect.objectContaining({ momentLocale: 'en', antd: expect.any(Object) }),
    );
    expect(register).toHaveBeenNthCalledWith(
      2,
      'zh-CN',
      { 'menu.language': '语言管理' },
      expect.objectContaining({ momentLocale: 'zh-cn', antd: expect.any(Object) }),
    );
  });
});
