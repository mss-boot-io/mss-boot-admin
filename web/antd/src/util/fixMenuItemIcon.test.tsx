import { DashboardOutlined, UserSwitchOutlined } from '@ant-design/icons';
import type { MenuDataItem } from '@ant-design/pro-components';
import type { ReactElement } from 'react';
import fixMenuItemIcon from './fixMenuItemIcon';

describe('fixMenuItemIcon', () => {
  it('resolves seeded icons recursively and preserves external targets', () => {
    const menus = [
      {
        icon: 'dashboard',
        path: 'https://docs.mss-boot-io.top',
        children: [{ icon: 'userSwitch', path: '/users' }],
      },
    ] as MenuDataItem[];

    const result = fixMenuItemIcon(menus);
    const parentIcon = result[0].icon as ReactElement;
    const childIcon = result[0].children?.[0].icon as ReactElement;

    expect(result[0].target).toBe('_blank');
    expect(parentIcon.type).toBe(DashboardOutlined);
    expect(childIcon.type).toBe(UserSwitchOutlined);
  });

  it('leaves unregistered icon names unset', () => {
    const menus = [{ icon: 'unregisteredIcon', path: '/custom' }] as MenuDataItem[];

    const result = fixMenuItemIcon(menus);

    expect(result[0].icon).toBeUndefined();
  });
});
