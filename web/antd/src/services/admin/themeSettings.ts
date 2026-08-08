import { deleteAppConfigsTheme, getAppConfigsGroup, putAppConfigsGroup } from './appConfig';
import { deleteUserConfigsTheme, getUserConfigsGroup, putUserConfigsGroup } from './userConfig';
import {
  parseThemeScopeResource,
  THEME_SETTING_KEYS,
  type ThemePatch,
  type ThemeScopeResource,
  type ThemeSettingKey,
  type ThemeSettingsScope,
} from '@/utils/themeSettings';

export type { ThemeSettingsScope } from '@/utils/themeSettings';

type ThemeGroupClient = {
  get: () => Promise<unknown>;
  put: (patch: ThemePatch, options?: Record<string, unknown>) => Promise<unknown>;
  delete: (options?: Record<string, unknown>) => Promise<unknown>;
};

export type ThemeSettingsAdapter = {
  scope: ThemeSettingsScope;
  load: () => Promise<ThemeScopeResource>;
  patch: (patch: ThemePatch, base: ThemeScopeResource) => Promise<ThemeScopeResource>;
  reset: (
    keys: readonly ThemeSettingKey[] | undefined,
    base: ThemeScopeResource,
  ) => Promise<ThemeScopeResource>;
};

export class ThemeRevisionConflictError extends Error {
  current?: ThemeScopeResource;

  constructor(current?: ThemeScopeResource) {
    super('Theme settings changed in another session');
    this.name = 'ThemeRevisionConflictError';
    this.current = current;
  }
}

const getStatus = (error: any) => error?.response?.status ?? error?.status;
const THEME_REVISION_CONFLICT_CODE = 'THEME_REVISION_CONFLICT';
export const THEME_CONTRACT_MEDIA_TYPE = 'application/vnd.mss.theme.v1+json';
const themeRequestOptions = {
  skipErrorHandler: true,
  headers: { Accept: THEME_CONTRACT_MEDIA_TYPE },
};

const isRevisionConflictResponse = (error: any) => {
  const code = error?.info?.errorCode ?? error?.response?.data?.errorCode;
  return getStatus(error) === 412 || code === 412 || code === THEME_REVISION_CONFLICT_CODE;
};

const getConflictCurrent = (
  error: any,
  scope: ThemeSettingsScope,
): ThemeScopeResource | undefined => {
  const value =
    error?.info?.data?.current ??
    error?.response?.data?.data?.current ??
    error?.response?.data?.current;
  if (!value) {
    return undefined;
  }
  try {
    return parseThemeScopeResource(value, scope);
  } catch {
    return undefined;
  }
};

export function isThemeRevisionConflictError(error: unknown): error is ThemeRevisionConflictError {
  return error instanceof ThemeRevisionConflictError;
}

export function formatThemeETag(resource: ThemeScopeResource) {
  return `"theme-${resource.scope}-${resource.revision}"`;
}

const mutationOptions = (base: ThemeScopeResource) =>
  base.versioned
    ? {
        skipErrorHandler: true,
        headers: {
          Accept: THEME_CONTRACT_MEDIA_TYPE,
          'If-Match': formatThemeETag(base),
        },
      }
    : themeRequestOptions;

const defaultClients: Record<ThemeSettingsScope, ThemeGroupClient> = {
  application: {
    get: () => getAppConfigsGroup({ group: 'theme' }, themeRequestOptions),
    put: (patch, options) => putAppConfigsGroup({ group: 'theme' }, { data: patch }, options),
    delete: (options) => deleteAppConfigsTheme(options),
  },
  user: {
    get: () => getUserConfigsGroup({ group: 'theme' }, themeRequestOptions),
    put: (patch, options) => putUserConfigsGroup({ group: 'theme' }, { data: patch }, options),
    delete: (options) => deleteUserConfigsTheme(options),
  },
};

export function createThemeSettingsAdapter(
  scope: ThemeSettingsScope,
  client: ThemeGroupClient = defaultClients[scope],
): ThemeSettingsAdapter {
  const load = async () => parseThemeScopeResource(await client.get(), scope);

  const mutate = async (request: () => Promise<unknown>): Promise<ThemeScopeResource> => {
    try {
      const resource = parseThemeScopeResource(await request(), scope);
      // Pre-revision servers return an empty success body. Read the canonical
      // scope instead of predicting the result client-side during rolling deploys.
      return resource.versioned ? resource : load();
    } catch (error) {
      if (isRevisionConflictResponse(error)) {
        throw new ThemeRevisionConflictError(getConflictCurrent(error, scope));
      }
      throw error;
    }
  };

  return {
    scope,
    load,
    patch: async (patch, base) => mutate(() => client.put(patch, mutationOptions(base))),
    reset: async (keys, base) => {
      if (!keys || keys.length === 0 || keys.length === THEME_SETTING_KEYS.length) {
        return mutate(() => client.delete(mutationOptions(base)));
      }
      return mutate(() =>
        client.put(
          Object.fromEntries(keys.map((key) => [key, null])) as ThemePatch,
          mutationOptions(base),
        ),
      );
    },
  };
}
