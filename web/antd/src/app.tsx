import Footer from '@/components/Footer';
import { Question, SelectLang } from '@/components/RightContent';
import { LinkOutlined, UserOutlined } from '@ant-design/icons';
import { SettingDrawer } from '@ant-design/pro-components';
import { RunTimeLayoutConfig } from '@umijs/max';
import { addLocale, FormattedMessage, history, Link } from '@umijs/max';
import { errorConfig } from './requestErrorConfig';
import React from 'react';
import { AvatarDropdown, AvatarName } from './components/RightContent/AvatarDropdown';
import { getUserDisplayName } from './components/RightContent/userDisplayName';
import { getUserUserInfo } from './services/admin/user';
import { requestAuthorizedMenu } from './utils/requestAuthorizedMenu';
import fixMenuItemIcon from './util/fixMenuItemIcon';
import { MenuDataItem } from '@ant-design/pro-components';
import { getCachedLanguages } from './services/admin/language';
import NoticeIconView from './components/NoticeIcon';
import AuthorizedMenuSearch from './components/AuthorizedMenuSearch';
import ThemeRuntimeBridge from './components/MssBoot/ThemeRuntimeBridge';
import PermissionFreshnessBridge from './components/MssBoot/PermissionFreshnessBridge';
import ForbiddenPage from './pages/403';
import { getAppConfigsProfile } from '@/services/admin/appConfig';
import { getUserConfigsProfile } from '@/services/admin/userConfig';
import { purgeLegacyOAuthStorage } from '@/utils/oauth';
import { isPublicRoute, PUBLIC_ROUTE_PATHS } from '@/utils/routeAccess';
import { clearAuthStorage, clearNonPersistentAuthStorage, getAuthToken } from '@/utils/authStorage';
import {
  applyThemeProfiles,
  buildLayoutSettings,
  getThemeScopeResource,
  markThemeScopeDegraded,
  normalizeThemeOverrides,
  type ThemeRuntimeCoordinatorState,
  type ThemeRuntimeState,
} from '@/utils/themeSettings';
import {
  applyThemeFirstPaintHint,
  clearThemeIdentitySession,
  ensureThemeAuthSession,
  isThemeBootstrapIdentityActive,
  readThemeBootstrapProfiles,
  writeThemeSnapshot,
} from '@/utils/themeSession';

const isDev = process.env.NODE_ENV === 'development';
const loginPath = PUBLIC_ROUTE_PATHS.login;
const AUTH_BOOTSTRAP_MAX_ATTEMPTS = 3;

type AdminInitialState = ThemeRuntimeState & {
  appConfig?: Record<string, Record<string, any>>;
  userConfig?: Record<string, Record<string, any>>;
  themeRuntime?: ThemeRuntimeCoordinatorState;
  currentUser?: API.User;
  permissionRefreshVersion?: number;
  loading?: boolean;
  fetchUserInfo?: () => Promise<API.User | undefined>;
};

const getAuthRedirect = (location: { pathname: string; search: string; hash: string }) => {
  const redirect =
    location.pathname === '/'
      ? '/workplace'
      : `${location.pathname}${location.search}${location.hash}`;
  return `${loginPath}?redirect=${encodeURIComponent(redirect)}`;
};

const withTimeout = async <T,>(request: () => Promise<T>, timeoutMs = 4000) => {
  return Promise.race<T | undefined>([
    request().catch(() => undefined),
    new Promise<undefined>((resolve) => {
      setTimeout(() => resolve(undefined), timeoutMs);
    }),
  ]);
};

/**
 * @see  https://umijs.org/zh-CN/plugins/plugin-initial-state
 * */
const createIdentityRaceFallback = (
  fetchUserInfo: () => Promise<API.User | undefined>,
): AdminInitialState => {
  const cachedTheme = readThemeBootstrapProfiles();
  let state = applyThemeProfiles<AdminInitialState>(
    { fetchUserInfo },
    cachedTheme.appConfig,
    undefined,
  );
  state = markThemeScopeDegraded(state, 'application');
  state = markThemeScopeDegraded(state, 'user');
  return state;
};

export async function getInitialState(identityAttempt = 0): Promise<AdminInitialState> {
  // Apply only the seven-field, versioned browser snapshot before React mounts.
  // Authoritative profile requests below always reconcile it before persistence.
  if (identityAttempt === 0) {
    applyThemeFirstPaintHint();
    purgeLegacyOAuthStorage();
  }
  const cachedTheme = readThemeBootstrapProfiles();

  const fetchUserInfo = async () => {
    return withTimeout(() =>
      getUserUserInfo({
        skipErrorHandler: true,
      }),
    );
  };

  const { location } = history;
  const isLoginRoute = location.pathname === loginPath;
  const isPublicPage = isPublicRoute(location.pathname);
  if (isLoginRoute) {
    clearNonPersistentAuthStorage();
  }
  const token = getAuthToken();

  if (!token || isPublicPage) {
    if (!isPublicPage) {
      history.replace(getAuthRedirect(location));
    }
    // The public application profile is the anonymous theme authority. Resolve
    // it before the login/register shell renders so dark deployments do not
    // flash the code default and then repaint after the page mounts.
    const authoritativeAppConfig = await withTimeout(
      () => getAppConfigsProfile({ skipErrorHandler: true }),
      2500,
    );
    const appConfig = authoritativeAppConfig || cachedTheme.appConfig;
    let state = applyThemeProfiles<AdminInitialState>(
      {
        fetchUserInfo,
      },
      appConfig,
      undefined,
    );
    if (authoritativeAppConfig) {
      await writeThemeSnapshot(
        getThemeScopeResource(authoritativeAppConfig, 'application'),
        undefined,
        undefined,
        { authoritativePrevious: cachedTheme.application },
      );
    } else {
      state = markThemeScopeDegraded(state, 'application');
    }
    return {
      ...state,
      fetchUserInfo,
    };
  }

  const expectedToken = token;
  const authSessionId = ensureThemeAuthSession({
    persistent: window.localStorage.getItem('token') === expectedToken,
  });
  const authenticatedCachedTheme = readThemeBootstrapProfiles();
  const identityIsActive = () =>
    isThemeBootstrapIdentityActive(expectedToken, authSessionId, getAuthToken());
  const retryAfterIdentityRace = () =>
    identityAttempt + 1 < AUTH_BOOTSTRAP_MAX_ATTEMPTS
      ? getInitialState(identityAttempt + 1)
      : createIdentityRaceFallback(fetchUserInfo);

  if (!identityIsActive()) {
    return retryAfterIdentityRace();
  }
  const currentUser = await fetchUserInfo();
  if (!identityIsActive()) {
    return retryAfterIdentityRace();
  }
  if (!currentUser) {
    clearAuthStorage();
    clearThemeIdentitySession();
    history.replace(getAuthRedirect(location));
    return {
      fetchUserInfo,
      settings: buildLayoutSettings(),
    };
  }

  // These resources are independent once authentication succeeds. Loading them
  // serially turns a slow network into a compounded first-page delay.
  const [languageData, authoritativeAppConfig, authoritativeUserConfig] = await Promise.all([
    withTimeout(() => getCachedLanguages(), 2500),
    withTimeout(() => getAppConfigsProfile({ skipErrorHandler: true }), 2500),
    withTimeout(() => getUserConfigsProfile({ skipErrorHandler: true }), 2500),
  ]);

  if (!identityIsActive()) {
    return retryAfterIdentityRace();
  }

  const appConfig = authoritativeAppConfig || authenticatedCachedTheme.appConfig;
  const userConfig = authoritativeUserConfig || authenticatedCachedTheme.userConfig;

  if (languageData?.data) {
    languageData.data.forEach((item) => {
      const obj = {};
      item.defines?.forEach((define) => {
        // @ts-ignore
        obj[`${define.group}.${define.key}`] = define.value;
      });

      const importPath = item.name!.replace('-', '_');
      //转成小写
      const momentLocale = item.name!.toLowerCase();

      addLocale(item.name!, obj, {
        momentLocale: momentLocale,
        // @ts-ignore
        antd: import(`antd/es/locale/${importPath}`).default,
      });
    });
  }

  let themedState = applyThemeProfiles<AdminInitialState>(
    {
      appConfig,
      userConfig,
      settings: buildLayoutSettings(appConfig, userConfig),
    } as AdminInitialState,
    appConfig,
    userConfig,
  );
  themedState = {
    ...themedState,
    themeRuntime: {
      schemaVersion: 1,
      layers: themedState.themeRuntime?.layers || {},
      ...themedState.themeRuntime,
      authSessionId,
    },
  };
  if (authoritativeAppConfig) {
    await writeThemeSnapshot(
      getThemeScopeResource(authoritativeAppConfig, 'application'),
      undefined,
      undefined,
      { authoritativePrevious: authenticatedCachedTheme.application },
    );
  } else {
    themedState = markThemeScopeDegraded(themedState, 'application');
  }
  if (authoritativeUserConfig && authSessionId) {
    await writeThemeSnapshot(
      getThemeScopeResource(authoritativeUserConfig, 'user'),
      authSessionId,
      undefined,
      { authoritativePrevious: authenticatedCachedTheme.user },
    );
  } else {
    themedState = markThemeScopeDegraded(themedState, 'user');
  }

  if (!identityIsActive()) {
    return retryAfterIdentityRace();
  }

  if (token && !isPublicPage) {
    try {
      return {
        ...themedState,
        fetchUserInfo,
        currentUser,
      };
    } catch (error) {
      clearAuthStorage();
      if (location.pathname !== loginPath) {
        history.replace(getAuthRedirect(location));
      }
      return {
        ...themedState,
        fetchUserInfo,
      };
    }
  }

  return {
    ...themedState,
    fetchUserInfo,
  };
}

// ProLayout 支持的api https://procomponents.ant.design/components/layout
export const layout: RunTimeLayoutConfig = ({ initialState, setInitialState }) => {
  return {
    title: initialState?.appConfig?.base?.websiteName || 'mss-boot-admin',
    menu: {
      locale: true,
      request: () =>
        requestAuthorizedMenu(
          initialState?.currentUser,
          initialState?.permissionRefreshVersion || 0,
        ),
      params: {
        permissionRefreshVersion: initialState?.permissionRefreshVersion || 0,
      },
    },
    actionsRender: () => [
      <AuthorizedMenuSearch
        key="search"
        identity={initialState?.currentUser}
        permissionRefreshVersion={initialState?.permissionRefreshVersion || 0}
      />,
      <NoticeIconView key="notice" />,
      <Question key="doc" />,
      <SelectLang key="SelectLang" />,
    ],
    avatarProps: {
      shape: 'circle',
      icon: <UserOutlined />,
      src: initialState?.currentUser?.avatar || undefined,
      title: <AvatarName />,
      render: (_, avatarChildren) => {
        return <AvatarDropdown menu={true}>{avatarChildren}</AvatarDropdown>;
      },
    },
    waterMarkProps: {
      content: getUserDisplayName(initialState?.currentUser),
      gapX: 220,
      gapY: 180,
      fontSize: 14,
      fontColor:
        initialState?.settings?.navTheme === 'realDark'
          ? 'rgba(255, 255, 255, 0.06)'
          : 'rgba(0, 0, 0, 0.055)',
    },
    footerRender: () => <Footer />,
    unAccessible: <ForbiddenPage />,
    onPageChange: () => {
      const { location } = history;
      const token = getAuthToken();
      if (!initialState?.currentUser && !token && !isPublicRoute(location.pathname)) {
        history.replace(getAuthRedirect(location));
      }
    },
    links: isDev
      ? [
          <Link key="openapi" to="/umi/plugin/openapi" target="_blank">
            <LinkOutlined />
            <span>
              OpenAPI <FormattedMessage id="app.documentation" defaultMessage="文档" />
            </span>
          </Link>,
        ]
      : [],
    menuDataRender: (menuData: MenuDataItem[]) => fixMenuItemIcon(menuData),
    childrenRender: (children) => {
      return (
        <>
          {children}
          <PermissionFreshnessBridge />
          <ThemeRuntimeBridge />
          {isDev && (
            <SettingDrawer
              disableUrlParams
              drawerProps={{
                title: (
                  <FormattedMessage
                    id="pages.theme.settings.preview"
                    defaultMessage="Development-only temporary preview"
                  />
                ),
              }}
              enableDarkTheme
              settings={initialState?.settings}
              onSettingChange={(settings) => {
                setInitialState((preInitialState) => ({
                  ...preInitialState,
                  settings: {
                    ...buildLayoutSettings(preInitialState?.appConfig, preInitialState?.userConfig),
                    ...normalizeThemeOverrides(settings),
                  },
                }));
              }}
            />
          )}
        </>
      );
    },
    ...initialState?.settings,
  };
};

/**
 * @name request 配置，可以配置错误处理
 * 它基于 axios 和 ahooks 的 useRequest 提供了一套统一的网络请求和错误处理方案。
 * @doc https://umijs.org/docs/max/request#配置
 */
export const request = {
  ...errorConfig,
};
