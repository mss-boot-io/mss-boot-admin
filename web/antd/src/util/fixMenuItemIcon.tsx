import {
  ApartmentOutlined,
  AuditOutlined,
  ClusterOutlined,
  DashboardOutlined,
  DesktopOutlined,
  ExperimentOutlined,
  FormOutlined,
  IdcardOutlined,
  InboxOutlined,
  MenuOutlined,
  MessageOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  SmileOutlined,
  TeamOutlined,
  ToolOutlined,
  TranslationOutlined,
  UnorderedListOutlined,
  UserOutlined,
  UserSwitchOutlined,
  WalletOutlined,
} from '@ant-design/icons';
import type { MenuDataItem } from '@ant-design/pro-components';
import React from 'react';

const menuIconMap: Record<string, typeof DashboardOutlined> = {
  ApartmentOutlined,
  AuditOutlined,
  ClusterOutlined,
  DashboardOutlined,
  DesktopOutlined,
  ExperimentOutlined,
  FormOutlined,
  IdcardOutlined,
  InboxOutlined,
  MenuOutlined,
  MessageOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  SmileOutlined,
  TeamOutlined,
  ToolOutlined,
  TranslationOutlined,
  UnorderedListOutlined,
  UserOutlined,
  UserSwitchOutlined,
  WalletOutlined,
};

const getMenuIcon = (icon: string, iconType: string) => {
  const normalizedIcon = icon.trim();
  if (!normalizedIcon) {
    return undefined;
  }

  const iconName =
    normalizedIcon.slice(0, 1).toLocaleUpperCase() + normalizedIcon.slice(1) + iconType;
  return menuIconMap[iconName] || menuIconMap[normalizedIcon];
};

// Menu icons are strings from the API. Keep this registry intentionally small so that
// dynamically looking up a name cannot pull the whole @ant-design/icons catalog into the entry chunk.
const fixMenuItemIcon = (menus: MenuDataItem[], iconType = 'Outlined'): MenuDataItem[] => {
  menus.forEach((item) => {
    const { icon, children } = item;
    if (item.path?.indexOf('http') === 0) {
      item.target = '_blank';
    }
    if (typeof icon === 'string') {
      const Icon = getMenuIcon(icon, iconType);
      if (Icon) {
        item.icon = React.createElement(Icon);
      } else {
        item.icon = undefined;
      }
    }
    if (children && children.length > 0) {
      item.children = fixMenuItemIcon(children, iconType);
    }
  });
  return menus;
};

export default fixMenuItemIcon;
