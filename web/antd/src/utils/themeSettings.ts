import type { Settings as LayoutSettings } from '@ant-design/pro-components';
import defaultSettings from '../../config/defaultSettings';

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

export type ThemeSettings = {
  navTheme: 'light' | 'realDark';
  colorPrimary: string;
  layout: 'side' | 'top' | 'mix';
  contentWidth: 'Fluid' | 'Fixed';
  fixedHeader: boolean;
  fixSiderbar: boolean;
  colorWeak: boolean;
};

export type ThemeOverrides = Partial<ThemeSettings>;
export type ThemePatch = Partial<Record<ThemeSettingKey, ThemeSettings[ThemeSettingKey] | null>>;
export type ConfigProfile = Record<string, Record<string, unknown>>;
export type ThemeSettingsScope = 'application' | 'user';
export type ThemeRevision = string;

export type ThemeResourceMeta = {
  v: 1;
  scope: ThemeSettingsScope;
  revision: ThemeRevision;
};

export type ThemeScopeResource = {
  scope: ThemeSettingsScope;
  revision: ThemeRevision;
  overrides: ThemeOverrides;
  /** False only while interoperating with a pre-revision backend response. */
  versioned: boolean;
};

export type ThemeRuntimeCoordinatorState = {
  schemaVersion: 1;
  layers: Partial<Record<ThemeSettingsScope, ThemeScopeResource>>;
  authSessionId?: string;
  degradedScopes?: ThemeSettingsScope[];
};
export type RuntimeLayoutSettings = Partial<LayoutSettings> &
  ThemeSettings & {
    title?: string;
    logo?: string;
  };

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

export const isThemeRecord = isRecord;

export function normalizeThemeRevision(value: unknown): ThemeRevision | undefined {
  if (typeof value !== 'string' || !/^\d+$/.test(value)) {
    return undefined;
  }
  return value.replace(/^0+(?=\d)/, '');
}

export function compareThemeRevisions(left: ThemeRevision, right: ThemeRevision): -1 | 0 | 1 {
  const normalizedLeft = normalizeThemeRevision(left);
  const normalizedRight = normalizeThemeRevision(right);
  if (normalizedLeft === undefined || normalizedRight === undefined) {
    throw new Error('Theme revision must be a decimal string');
  }
  if (normalizedLeft.length !== normalizedRight.length) {
    return normalizedLeft.length < normalizedRight.length ? -1 : 1;
  }
  if (normalizedLeft === normalizedRight) {
    return 0;
  }
  return normalizedLeft < normalizedRight ? -1 : 1;
}

const normalizeBoolean = (value: unknown): boolean | undefined => {
  if (typeof value === 'boolean') {
    return value;
  }
  if (value === 'true') {
    return true;
  }
  if (value === 'false') {
    return false;
  }
  return undefined;
};

const normalizeEnum = <T extends string>(value: unknown, allowed: readonly T[]): T | undefined =>
  typeof value === 'string' && allowed.includes(value as T) ? (value as T) : undefined;

const normalizeColor = (value: unknown): string | undefined => {
  let color = value;
  if (
    typeof value === 'object' &&
    value !== null &&
    'toHexString' in value &&
    typeof value.toHexString === 'function'
  ) {
    color = value.toHexString();
  }
  if (typeof color !== 'string') {
    return undefined;
  }
  const normalized = color.trim().toLowerCase();
  return /^#[0-9a-f]{6}$/.test(normalized) ? normalized : undefined;
};

export const CODE_THEME_DEFAULTS: Readonly<ThemeSettings> = Object.freeze({
  navTheme: defaultSettings.navTheme === 'light' ? 'light' : 'realDark',
  colorPrimary: normalizeColor(defaultSettings.colorPrimary) || '#1890ff',
  layout:
    defaultSettings.layout === 'side' ||
    defaultSettings.layout === 'top' ||
    defaultSettings.layout === 'mix'
      ? defaultSettings.layout
      : 'mix',
  contentWidth: defaultSettings.contentWidth === 'Fixed' ? 'Fixed' : 'Fluid',
  fixedHeader: defaultSettings.fixedHeader === true,
  fixSiderbar: defaultSettings.fixSiderbar !== false,
  colorWeak: defaultSettings.colorWeak === true,
});

export function normalizeThemeOverrides(value: unknown): ThemeOverrides {
  if (!isRecord(value)) {
    return {};
  }

  const normalized: ThemeOverrides = {};
  const navTheme = normalizeEnum(value.navTheme, ['light', 'realDark']);
  const colorPrimary = normalizeColor(value.colorPrimary);
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

export function hasThemeOverride(overrides: ThemeOverrides, key: ThemeSettingKey) {
  return Object.prototype.hasOwnProperty.call(overrides, key);
}

export function areThemeOverridesEqual(left: ThemeOverrides, right: ThemeOverrides): boolean {
  return THEME_SETTING_KEYS.every(
    (key) =>
      hasThemeOverride(left, key) === hasThemeOverride(right, key) && left[key] === right[key],
  );
}

export function parseThemeScopeResource(
  value: unknown,
  expectedScope: ThemeSettingsScope,
): ThemeScopeResource {
  const record = isRecord(value) ? value : {};
  const meta = isRecord(record._meta) ? record._meta : undefined;
  if (!meta) {
    return {
      scope: expectedScope,
      revision: '0',
      overrides: normalizeThemeOverrides(record),
      versioned: false,
    };
  }

  if (meta.v !== 1 || meta.scope !== expectedScope) {
    throw new Error(`Invalid ${expectedScope} theme resource metadata`);
  }
  const revision = normalizeThemeRevision(meta.revision);
  if (revision === undefined) {
    throw new Error(`Invalid ${expectedScope} theme revision`);
  }
  return {
    scope: expectedScope,
    revision,
    overrides: normalizeThemeOverrides(record),
    versioned: true,
  };
}

export function serializeThemeScopeResource(resource: ThemeScopeResource): Record<string, unknown> {
  return {
    ...resource.overrides,
    ...(resource.versioned
      ? {
          _meta: {
            v: 1,
            scope: resource.scope,
            revision: resource.revision,
          } satisfies ThemeResourceMeta,
        }
      : {}),
  };
}

const applyOverrides = (settings: ThemeSettings, overrides: ThemeOverrides): ThemeSettings => {
  const next = { ...settings };
  THEME_SETTING_KEYS.forEach((key) => {
    const value = overrides[key];
    if (value !== undefined) {
      Object.assign(next, { [key]: value });
    }
  });
  return next;
};

export function resolveThemeSettings(
  codeDefaults: ThemeSettings | Readonly<ThemeSettings>,
  application?: unknown,
  user?: unknown,
): ThemeSettings {
  return applyOverrides(
    applyOverrides({ ...codeDefaults }, normalizeThemeOverrides(application)),
    normalizeThemeOverrides(user),
  );
}

export function getThemeOverrides(profile: unknown): ThemeOverrides {
  if (!isRecord(profile)) {
    return {};
  }
  return normalizeThemeOverrides(profile.theme);
}

export function getThemeScopeResource(
  profile: unknown,
  scope: ThemeSettingsScope,
): ThemeScopeResource {
  if (!isRecord(profile)) {
    return parseThemeScopeResource({}, scope);
  }
  return parseThemeScopeResource(profile.theme, scope);
}

export function setThemeOverrides(profile: unknown, theme: ThemeOverrides): ConfigProfile {
  const next = isRecord(profile) ? { ...profile } : {};
  return {
    ...(next as ConfigProfile),
    theme: { ...theme },
  };
}

export function setThemeScopeResource(
  profile: unknown,
  resource: ThemeScopeResource,
): ConfigProfile {
  const next = isRecord(profile) ? { ...profile } : {};
  return {
    ...(next as ConfigProfile),
    theme: serializeThemeScopeResource(resource),
  };
}

const getString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim() ? value : undefined;

export function buildLayoutSettings(
  appConfig?: unknown,
  userConfig?: unknown,
): RuntimeLayoutSettings {
  const appProfile = isRecord(appConfig) ? appConfig : {};
  const base = isRecord(appProfile.base) ? appProfile.base : {};
  return {
    ...(defaultSettings as unknown as Partial<LayoutSettings>),
    title: getString(base.websiteName) || getString(defaultSettings.title),
    logo: getString(base.websiteLogo) || getString(defaultSettings.logo),
    ...resolveThemeSettings(
      CODE_THEME_DEFAULTS,
      getThemeOverrides(appProfile),
      getThemeOverrides(userConfig),
    ),
  };
}

export type ThemeRuntimeState = {
  appConfig?: unknown;
  userConfig?: unknown;
  settings?: RuntimeLayoutSettings;
  themeRuntime?: ThemeRuntimeCoordinatorState;
};

export function getVerifiedThemeAuthSessionId(
  state: ThemeRuntimeState | undefined,
  activeStorageSessionId: string | undefined,
) {
  const verified = state?.themeRuntime?.authSessionId;
  return verified && verified === activeStorageSessionId ? verified : undefined;
}

export type ThemeReconcileStatus = 'applied' | 'duplicate' | 'stale' | 'same-revision-conflict';

export type ThemeReconcileResult<T extends ThemeRuntimeState> = {
  state: T;
  status: ThemeReconcileStatus;
};

export function reconcileThemeScopeResource<T extends ThemeRuntimeState>(
  state: T,
  resource: ThemeScopeResource,
  options: {
    allowLegacyReplace?: boolean;
    authSessionId?: string;
    authoritative?: boolean;
  } = {},
): ThemeReconcileResult<T> {
  const current = state.themeRuntime?.layers?.[resource.scope];
  if (current) {
    const comparison = compareThemeRevisions(resource.revision, current.revision);
    const replacingBootstrapHint =
      options.authoritative === true &&
      state.themeRuntime?.degradedScopes?.includes(resource.scope) === true;
    if (comparison < 0 && !replacingBootstrapHint) {
      return { state, status: 'stale' };
    }
    if (comparison === 0) {
      if (areThemeOverridesEqual(resource.overrides, current.overrides) && !options.authoritative) {
        return { state, status: 'duplicate' };
      }
      if (options.authoritative) {
        // A successful scope GET or mutation response is canonical and resolves
        // divergent same-revision payloads as well as a prior degraded marker.
      } else if (!resource.versioned && !current.versioned && options.allowLegacyReplace) {
        // A legacy backend has no ordering signal. Authoritative GET responses
        // may still replace the local legacy projection during rolling deploys.
      } else {
        return { state, status: 'same-revision-conflict' };
      }
    }
  }

  const appConfig =
    resource.scope === 'application'
      ? setThemeScopeResource(state.appConfig, resource)
      : state.appConfig;
  const userConfig =
    resource.scope === 'user'
      ? setThemeScopeResource(state.userConfig, resource)
      : state.userConfig;
  const layers = {
    ...(state.themeRuntime?.layers || {}),
    [resource.scope]: resource,
  };
  const degradedScopes = (state.themeRuntime?.degradedScopes || []).filter(
    (scope) => scope !== resource.scope,
  );

  return {
    status: 'applied',
    state: {
      ...state,
      appConfig,
      userConfig,
      settings: buildLayoutSettings(appConfig, userConfig),
      themeRuntime: {
        schemaVersion: 1,
        ...state.themeRuntime,
        layers,
        authSessionId: options.authSessionId ?? state.themeRuntime?.authSessionId,
        ...(degradedScopes.length > 0 ? { degradedScopes } : { degradedScopes: undefined }),
      },
    },
  };
}

function mergeProfileWithoutTheme(existing: unknown, incoming: unknown) {
  const base = isRecord(existing) ? existing : {};
  if (!isRecord(incoming)) return base;
  const nonThemeEntries = Object.entries(incoming).filter(([key]) => key !== 'theme');
  return { ...base, ...Object.fromEntries(nonThemeEntries) };
}

export function reconcileThemeProfile<T extends ThemeRuntimeState>(
  state: T,
  profile: unknown,
  scope: ThemeSettingsScope,
  options: {
    allowLegacyReplace?: boolean;
    authSessionId?: string;
    authoritative?: boolean;
  } = {},
): ThemeReconcileResult<T> {
  const base = {
    ...state,
    ...(scope === 'application'
      ? { appConfig: mergeProfileWithoutTheme(state.appConfig, profile) }
      : { userConfig: mergeProfileWithoutTheme(state.userConfig, profile) }),
  } as T;
  base.settings = buildLayoutSettings(base.appConfig, base.userConfig);
  return reconcileThemeScopeResource(base, getThemeScopeResource(profile, scope), options);
}

export function markThemeScopeDegraded<T extends ThemeRuntimeState>(
  state: T,
  scope: ThemeSettingsScope,
): T {
  const degradedScopes = Array.from(
    new Set([...(state.themeRuntime?.degradedScopes || []), scope]),
  );
  return {
    ...state,
    themeRuntime: {
      schemaVersion: 1,
      layers: state.themeRuntime?.layers || {},
      ...state.themeRuntime,
      degradedScopes,
    },
  };
}

export function applyThemeProfiles<T extends ThemeRuntimeState>(
  state: T,
  appConfig: unknown,
  userConfig: unknown,
): T {
  const application = getThemeScopeResource(appConfig, 'application');
  const user = userConfig === undefined ? undefined : getThemeScopeResource(userConfig, 'user');
  return {
    ...state,
    appConfig,
    userConfig,
    settings: buildLayoutSettings(appConfig, userConfig),
    themeRuntime: {
      schemaVersion: 1,
      ...state.themeRuntime,
      layers: {
        application,
        ...(user ? { user } : {}),
      },
    },
  };
}

export function clearUserThemeProfile<T extends ThemeRuntimeState>(state: T): T {
  const application = state.themeRuntime?.layers.application;
  const degradedScopes = (state.themeRuntime?.degradedScopes || []).filter(
    (scope) => scope !== 'user',
  );
  return {
    ...state,
    userConfig: undefined,
    settings: buildLayoutSettings(state.appConfig, undefined),
    themeRuntime: {
      schemaVersion: 1,
      ...state.themeRuntime,
      layers: application ? { application } : {},
      authSessionId: undefined,
      ...(degradedScopes.length > 0 ? { degradedScopes } : { degradedScopes: undefined }),
    },
  };
}
