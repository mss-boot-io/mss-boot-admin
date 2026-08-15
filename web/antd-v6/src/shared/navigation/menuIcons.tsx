import AuditOutlined from '@ant-design/icons/AuditOutlined';
import SafetyCertificateOutlined from '@ant-design/icons/SafetyCertificateOutlined';
import SettingOutlined from '@ant-design/icons/SettingOutlined';
import ShopOutlined from '@ant-design/icons/ShopOutlined';
import SmileOutlined from '@ant-design/icons/SmileOutlined';
import TranslationOutlined from '@ant-design/icons/TranslationOutlined';
import UnorderedListOutlined from '@ant-design/icons/UnorderedListOutlined';
import UserSwitchOutlined from '@ant-design/icons/UserSwitchOutlined';
import type { MenuDataItem } from '@ant-design/pro-components';
import { type ComponentType, createElement } from 'react';

const registeredIcons: Readonly<Record<string, ComponentType>> = {
  audit: AuditOutlined,
  safetyCertificate: SafetyCertificateOutlined,
  setting: SettingOutlined,
  shop: ShopOutlined,
  smile: SmileOutlined,
  translation: TranslationOutlined,
  unorderedList: UnorderedListOutlined,
  userSwitch: UserSwitchOutlined,
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
