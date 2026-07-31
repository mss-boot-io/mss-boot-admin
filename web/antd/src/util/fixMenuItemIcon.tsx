import React from 'react';
import * as allIcons from '@ant-design/icons';
import { MenuDataItem } from '@ant-design/pro-components';

// FIX从接口获取菜单时icon为string类型
const fixMenuItemIcon = (menus: MenuDataItem[], iconType = 'Outlined'): MenuDataItem[] => {
  menus.forEach((item) => {
    const { icon, children } = item;
    if (item.path?.indexOf('http') === 0) {
      item.target = '_blank';
    }
    if (typeof icon === 'string') {
      const fixIconName = icon.slice(0, 1).toLocaleUpperCase() + icon.slice(1) + iconType;
      // @ts-ignore
      if (allIcons[fixIconName] || allIcons[icon]) {
        // @ts-ignore
        item.icon = React.createElement(allIcons[fixIconName] || allIcons[icon]);
      }
    }
    // @ts-ignore
    if (children && children.length > 0) {
      item.children = fixMenuItemIcon(children);
    }
  });
  return menus;
};

export default fixMenuItemIcon;
