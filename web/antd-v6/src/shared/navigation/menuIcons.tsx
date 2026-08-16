import ApartmentOutlined from '@ant-design/icons/ApartmentOutlined';
import AuditOutlined from '@ant-design/icons/AuditOutlined';
import ClusterOutlined from '@ant-design/icons/ClusterOutlined';
import DashboardOutlined from '@ant-design/icons/DashboardOutlined';
import DeploymentUnitOutlined from '@ant-design/icons/DeploymentUnitOutlined';
import DesktopOutlined from '@ant-design/icons/DesktopOutlined';
import FileTextOutlined from '@ant-design/icons/FileTextOutlined';
import IdcardOutlined from '@ant-design/icons/IdcardOutlined';
import InboxOutlined from '@ant-design/icons/InboxOutlined';
import MenuOutlined from '@ant-design/icons/MenuOutlined';
import MessageOutlined from '@ant-design/icons/MessageOutlined';
import SafetyCertificateOutlined from '@ant-design/icons/SafetyCertificateOutlined';
import SafetyOutlined from '@ant-design/icons/SafetyOutlined';
import SettingOutlined from '@ant-design/icons/SettingOutlined';
import ShopOutlined from '@ant-design/icons/ShopOutlined';
import SmileOutlined from '@ant-design/icons/SmileOutlined';
import TeamOutlined from '@ant-design/icons/TeamOutlined';
import TranslationOutlined from '@ant-design/icons/TranslationOutlined';
import UnorderedListOutlined from '@ant-design/icons/UnorderedListOutlined';
import UserOutlined from '@ant-design/icons/UserOutlined';
import UserSwitchOutlined from '@ant-design/icons/UserSwitchOutlined';
import WalletOutlined from '@ant-design/icons/WalletOutlined';
import WarningOutlined from '@ant-design/icons/WarningOutlined';
import type { MenuDataItem } from '@ant-design/pro-components';
import { type ComponentType, createElement } from 'react';

const registeredIcons: Readonly<Record<string, ComponentType>> = {
  apartment: ApartmentOutlined,
  audit: AuditOutlined,
  cluster: ClusterOutlined,
  dashboard: DashboardOutlined,
  deploymentUnit: DeploymentUnitOutlined,
  desktop: DesktopOutlined,
  fileText: FileTextOutlined,
  idcard: IdcardOutlined,
  inbox: InboxOutlined,
  menu: MenuOutlined,
  message: MessageOutlined,
  safety: SafetyOutlined,
  safetyCertificate: SafetyCertificateOutlined,
  setting: SettingOutlined,
  shop: ShopOutlined,
  smile: SmileOutlined,
  team: TeamOutlined,
  translation: TranslationOutlined,
  unorderedList: UnorderedListOutlined,
  user: UserOutlined,
  userSwitch: UserSwitchOutlined,
  wallet: WalletOutlined,
  warning: WarningOutlined,
};

function resolveIcon(icon: MenuDataItem['icon']): MenuDataItem['icon'] {
  if (typeof icon !== 'string') return icon;
  const normalized = icon.trim().replace(/Outlined$/, '');
  if (!normalized) return undefined;
  const key = `${normalized.slice(0, 1).toLowerCase()}${normalized.slice(1)}`;
  const Icon = registeredIcons[key];
  return Icon ? createElement(Icon) : undefined;
}

/**
 * Resolve only the backend icon keys used by routes compiled into V6.
 * An explicit registry avoids rendering untrusted strings and avoids bundling
 * the complete icon catalog. Unknown keys fail closed without visible text.
 */
export function resolveMenuIcons(items: MenuDataItem[]): MenuDataItem[] {
  return items.map((item) => ({
    ...item,
    icon: resolveIcon(item.icon),
    children: item.children ? resolveMenuIcons(item.children) : undefined,
  }));
}
