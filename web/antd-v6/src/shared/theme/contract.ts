import type { ProLayoutProps } from '@ant-design/pro-components';
import defaultSettings from '../../../config/defaultSettings';

export const THEME_SETTING_KEYS = [
  'navTheme',
  'colorPrimary',
  'layout',
  'contentWidth',
  'fixedHeader',
  'fixSiderbar',
  'colorWeak',
] as const;

export type ThemeSettingKey = (typeof THEME_SETTING_KEYS)[number];
export type ThemeScope = 'application' | 'user';

export interface ThemeSettings {
  navTheme: 'light' | 'realDark';
  colorPrimary: string;
  layout: 'side' | 'top' | 'mix';
  contentWidth: 'Fluid' | 'Fixed';
  fixedHeader: boolean;
  fixSiderbar: boolean;
  colorWeak: boolean;
}

export type ThemeOverrides = Partial<ThemeSettings>;
export type ThemePatch = Partial<Record<ThemeSettingKey, ThemeSettings[ThemeSettingKey] | null>>;
export type ThemeSource = 'code' | ThemeScope;

export interface ThemeScopeResource {
  scope: ThemeScope;
  revision: string;
  overrides: ThemeOverrides;
  versioned: boolean;
}

export interface ResolvedTheme {
  settings: ThemeSettings;
  sources: Record<ThemeSettingKey, ThemeSource>;
}

export interface ApplicationProfile {
  base: Record<string, unknown>;
  security: Record<string, unknown>;
  theme: ThemeScopeResource;
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

const normalizeBoolean = (value: unknown): boolean | undefined => {
  if (typeof value === 'boolean') return value;
  if (value === 'true') return true;
  if (value === 'false') return false;
  return undefined;
};

const normalizeEnum = <T extends string>(value: unknown, allowed: readonly T[]): T | undefined =>
  typeof value === 'string' && allowed.includes(value as T) ? (value as T) : undefined;

export function normalizeThemeColor(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined;
  const normalized = value.trim().toLowerCase();
  return /^#[0-9a-f]{6}$/.test(normalized) ? normalized : undefined;
}

/**
 * V6 intentionally uses the new application's design baseline for the code
 * layer. Existing application and personal overrides retain their exact
 * precedence above it, so this is a visual modernization rather than a data
 * migration or a copied V5 Less default.
 */
export const CODE_THEME_DEFAULTS: Readonly<ThemeSettings> = Object.freeze({
  navTheme: defaultSettings.navTheme,
  colorPrimary: normalizeThemeColor(defaultSettings.colorPrimary) ?? '#1677ff',
  layout: defaultSettings.layout,
  contentWidth: defaultSettings.contentWidth,
  fixedHeader: defaultSettings.fixedHeader,
  fixSiderbar: defaultSettings.fixSiderbar,
  colorWeak: defaultSettings.colorWeak,
});

export function normalizeThemeOverrides(value: unknown): ThemeOverrides {
  if (!isRecord(value)) return {};
  const normalized: ThemeOverrides = {};
  const navTheme = normalizeEnum(value.navTheme, ['light', 'realDark']);
  const colorPrimary = normalizeThemeColor(value.colorPrimary);
  const layout = normalizeEnum(value.layout, ['side', 'top', 'mix']);
  const contentWidth = normalizeEnum(value.contentWidth, ['Fluid', 'Fixed']);
  const fixedHeader = normalizeBoolean(value.fixedHeader);
  const fixSiderbar = normalizeBoolean(value.fixSiderbar);
  const colorWeak = normalizeBoolean(value.colorWeak);

  if (navTheme !== undefined) normalized.navTheme = navTheme;
  if (colorPrimary !== undefined) normalized.colorPrimary = colorPrimary;
  if (layout !== undefined) normalized.layout = layout;
  if (contentWidth !== undefined) normalized.contentWidth = contentWidth;
  if (fixedHeader !== undefined) normalized.fixedHeader = fixedHeader;
  if (fixSiderbar !== undefined) normalized.fixSiderbar = fixSiderbar;
  if (colorWeak !== undefined) normalized.colorWeak = colorWeak;
  return normalized;
}

export function normalizeThemeRevision(value: unknown): string | undefined {
  if (typeof value !== 'string' || !/^\d+$/.test(value)) return undefined;
  return value.replace(/^0+(?=\d)/, '');
}

export function compareThemeRevisions(left: string, right: string): -1 | 0 | 1 {
  const normalizedLeft = normalizeThemeRevision(left);
  const normalizedRight = normalizeThemeRevision(right);
  if (normalizedLeft === undefined || normalizedRight === undefined) {
    throw new Error('Theme revision must be a decimal string');
  }
  if (normalizedLeft.length !== normalizedRight.length) {
    return normalizedLeft.length < normalizedRight.length ? -1 : 1;
  }
  if (normalizedLeft === normalizedRight) return 0;
  return normalizedLeft < normalizedRight ? -1 : 1;
}

export function parseThemeScopeResource(value: unknown, scope: ThemeScope): ThemeScopeResource {
  const record = isRecord(value) ? value : {};
  const meta = isRecord(record._meta) ? record._meta : undefined;
  if (!meta) {
    return { scope, revision: '0', overrides: normalizeThemeOverrides(record), versioned: false };
  }
  const revision = normalizeThemeRevision(meta.revision);
  if (meta.v !== 1 || meta.scope !== scope || revision === undefined) {
    throw new Error(`Invalid ${scope} theme resource metadata`);
  }
  return { scope, revision, overrides: normalizeThemeOverrides(record), versioned: true };
}

export function parseApplicationProfile(value: unknown): ApplicationProfile {
  if (!isRecord(value)) throw new Error('Invalid public application profile');
  return {
    base: isRecord(value.base) ? { ...value.base } : {},
    security: isRecord(value.security) ? { ...value.security } : {},
    theme: parseThemeScopeResource(value.theme, 'application'),
  };
}

function hasOverride(overrides: ThemeOverrides, key: ThemeSettingKey): boolean {
  return Object.hasOwn(overrides, key);
}

export function resolveTheme(
  application?: ThemeScopeResource,
  user?: ThemeScopeResource,
  code: Readonly<ThemeSettings> = CODE_THEME_DEFAULTS,
): ResolvedTheme {
  const settings = { ...code };
  const sources = Object.fromEntries(THEME_SETTING_KEYS.map((key) => [key, 'code'])) as Record<
    ThemeSettingKey,
    ThemeSource
  >;

  for (const resource of [application, user]) {
    if (!resource) continue;
    for (const key of THEME_SETTING_KEYS) {
      if (!hasOverride(resource.overrides, key)) continue;
      Object.assign(settings, { [key]: resource.overrides[key] });
      sources[key] = resource.scope;
    }
  }
  return { settings, sources };
}

function profileString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined;
}

export function buildLayoutSettings(
  resolved: ThemeSettings,
  applicationBase?: Record<string, unknown>,
): Partial<ProLayoutProps> {
  return {
    ...defaultSettings,
    ...resolved,
    title: profileString(applicationBase?.websiteName) ?? defaultSettings.title,
    logo: profileString(applicationBase?.websiteLogo) ?? defaultSettings.logo,
  };
}
