import SmileOutlined from '@ant-design/icons/SmileOutlined';
import UserSwitchOutlined from '@ant-design/icons/UserSwitchOutlined';
import WalletOutlined from '@ant-design/icons/WalletOutlined';
import type { MenuDataItem } from '@ant-design/pro-components';
import type { ReactElement } from 'react';
import { describe, expect, it } from 'vitest';
import { resolveMenuIcons } from './menuIcons';

describe('resolveMenuIcons', () => {
  it('resolves registered backend icon keys recursively without mutating input', () => {
    const source: MenuDataItem[] = [
      {
        icon: 'smile',
        children: [{ icon: 'UserSwitchOutlined' }],
      },
    ];

    const result = resolveMenuIcons(source);
    const sourceRoot = source.at(0);
    const resultRoot = result.at(0);
    const child = resultRoot?.children?.at(0);
    if (!sourceRoot) throw new Error('source root menu is missing');
    if (!resultRoot) throw new Error('resolved root menu is missing');
    if (!child) throw new Error('resolved child menu is missing');

    expect((resultRoot.icon as ReactElement).type).toBe(SmileOutlined);
    expect((child.icon as ReactElement).type).toBe(UserSwitchOutlined);
    expect(sourceRoot.icon).toBe('smile');
    expect(sourceRoot.children?.at(0)?.icon).toBe('UserSwitchOutlined');
  });

  it('drops unknown icon keys instead of rendering them as menu text', () => {
    const result = resolveMenuIcons([{ icon: 'serverControlledUnknownIcon' }]);
    expect(result.at(0)?.icon).toBeUndefined();
  });

  it('resolves the operational task icon through an explicit public import', () => {
    const result = resolveMenuIcons([{ icon: 'wallet' }]);
    const taskMenu = result.at(0);
    if (!taskMenu) throw new Error('resolved task menu is missing');

    expect((taskMenu.icon as ReactElement).type).toBe(WalletOutlined);
  });
});
