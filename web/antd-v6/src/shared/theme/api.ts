import { request } from '@umijs/max';
import {
  type ApplicationProfile,
  parseApplicationProfile,
  parseThemeScopeResource,
  THEME_SETTING_KEYS,
  type ThemePatch,
  type ThemeScope,
  type ThemeScopeResource,
  type ThemeSettingKey,
} from './contract';

export const THEME_CONTRACT_MEDIA_TYPE = 'application/vnd.mss.theme.v1+json';

const endpoint = (scope: ThemeScope) =>
  scope === 'application' ? '/app-configs/theme' : '/user-configs/theme';

const canonicalHeaders = { Accept: THEME_CONTRACT_MEDIA_TYPE };

export interface ThemeRequestOptions {
  method: 'GET' | 'PUT' | 'DELETE';
  data?: unknown;
  headers?: Record<string, string>;
  skipErrorHandler: true;
}

export type ThemeRequestClient = (path: string, options: ThemeRequestOptions) => Promise<unknown>;

export interface ThemeAPI {
  loadApplicationProfile: () => Promise<ApplicationProfile>;
  loadThemeResource: (scope: ThemeScope) => Promise<ThemeScopeResource>;
  patchThemeResource: (
    scope: ThemeScope,
    patch: ThemePatch,
    base: ThemeScopeResource,
  ) => Promise<ThemeScopeResource>;
  resetThemeResource: (
    scope: ThemeScope,
    base: ThemeScopeResource,
    keys?: readonly ThemeSettingKey[],
  ) => Promise<ThemeScopeResource>;
}

export class ThemeRevisionConflictError extends Error {
  current?: ThemeScopeResource;

  constructor(current?: ThemeScopeResource) {
    super('Theme settings changed in another session');
    this.name = 'ThemeRevisionConflictError';
    this.current = current;
  }
}

export function formatThemeETag(resource: ThemeScopeResource): string {
  return `"theme-${resource.scope}-${resource.revision}"`;
}

function conflictResource(error: unknown, scope: ThemeScope): ThemeScopeResource | undefined {
  const value = (error as { response?: { data?: { data?: { current?: unknown } } } }).response?.data
    ?.data?.current;
  if (!value) return undefined;
  try {
    return parseThemeScopeResource(value, scope);
  } catch {
    return undefined;
  }
}

export function createThemeAPI(client: ThemeRequestClient): ThemeAPI {
  const loadThemeResource = async (scope: ThemeScope): Promise<ThemeScopeResource> => {
    const value = await client(endpoint(scope), {
      method: 'GET',
      headers: canonicalHeaders,
      skipErrorHandler: true,
    });
    return parseThemeScopeResource(value, scope);
  };

  const mutateTheme = async (
    scope: ThemeScope,
    base: ThemeScopeResource,
    method: 'PUT' | 'DELETE',
    patch?: ThemePatch,
  ): Promise<ThemeScopeResource> => {
    try {
      const value = await client(endpoint(scope), {
        method,
        data: method === 'PUT' ? { data: patch } : undefined,
        headers: {
          ...canonicalHeaders,
          ...(base.versioned ? { 'If-Match': formatThemeETag(base) } : {}),
        },
        skipErrorHandler: true,
      });
      const resource = parseThemeScopeResource(value, scope);
      return resource.versioned ? resource : loadThemeResource(scope);
    } catch (error) {
      if ((error as { response?: { status?: number } })?.response?.status === 412) {
        throw new ThemeRevisionConflictError(conflictResource(error, scope));
      }
      throw error;
    }
  };

  return {
    loadApplicationProfile: async () =>
      parseApplicationProfile(
        await client('/app-configs/profile', {
          method: 'GET',
          skipErrorHandler: true,
        }),
      ),
    loadThemeResource,
    patchThemeResource: (scope, patch, base) => mutateTheme(scope, base, 'PUT', patch),
    resetThemeResource: (scope, base, keys) => {
      if (!keys || keys.length === 0 || keys.length === THEME_SETTING_KEYS.length) {
        return mutateTheme(scope, base, 'DELETE');
      }
      return mutateTheme(
        scope,
        base,
        'PUT',
        Object.fromEntries(keys.map((key) => [key, null])) as ThemePatch,
      );
    },
  };
}

const defaultThemeAPI = createThemeAPI((path, options) => request<unknown>(path, options));

export const loadApplicationProfile = defaultThemeAPI.loadApplicationProfile;
export const loadThemeResource = defaultThemeAPI.loadThemeResource;
export const patchThemeResource = defaultThemeAPI.patchThemeResource;
export const resetThemeResource = defaultThemeAPI.resetThemeResource;
