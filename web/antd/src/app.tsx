import Footer from '@/components/Footer';
import { Question, SelectLang } from '@/components/RightContent';
import { LinkOutlined } from '@ant-design/icons';
import type { Settings as LayoutSettings } from '@ant-design/pro-components';
import { SettingDrawer } from '@ant-design/pro-components';
import { RunTimeLayoutConfig } from '@umijs/max';
import { addLocale, FormattedMessage, history, Link } from '@umijs/max';
import defaultSettings from '../config/defaultSettings';
import { errorConfig } from './requestErrorConfig';
import React from 'react';
import { AvatarDropdown, AvatarName } from './components/RightContent/AvatarDropdown';
import { getUserUserInfo } from './services/admin/user';
import { getMenuAuthorize } from './services/admin/menu';
import fixMenuItemIcon from './util/fixMenuItemIcon';
import { MenuDataItem } from '@ant-design/pro-components';
import { getLanguages } from './services/admin/language';
import NoticeIconView from './components/NoticeIcon';
import HeaderSearch from './components/HeaderSearch';
import { getAppConfigsProfile } from '@/services/admin/appConfig';
import { getUserConfigsProfile } from '@/services/admin/userConfig';

const isDev = process.env.NODE_ENV === 'development';
const loginPath = '/user/login';
const excludePath = [
  '/user/github-callback',
  '/user/lark-callback',
  '/user/register',
  '/user/callback/github',
  '/user/callback/lark',
];

const getAuthRedirect = (location: { pathname: string; search: string; hash: string }) => {
  const redirect =
    location.pathname === '/'
      ? '/workplace'
      : `${location.pathname}${location.search}${location.hash}`;
  return `${loginPath}?redirect=${encodeURIComponent(redirect)}`;
};

const clearAuthState = () => {
  localStorage.removeItem('token');
  localStorage.removeItem('token.expire');
  localStorage.removeItem('autoLogin');
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
export async function getInitialState(): Promise<{
  settings?: Partial<LayoutSettings>;
  appConfig?: Record<string, Record<string, string>>;
  userConfig?: Record<string, Record<string, string>>;
  currentUser?: API.User;
  loading?: boolean;
  fetchUserInfo?: () => Promise<API.User | undefined>;
}> {
  const fetchUserInfo = async () => {
    return withTimeout(() =>
      getUserUserInfo({
        skipErrorHandler: true,
      }),
    );
  };

  const { location } = history;
  const token = localStorage.getItem('token');
  const isLoginPage = location.pathname === loginPath || excludePath.includes(location.pathname);

  if (!token || isLoginPage) {
    if (!isLoginPage) {
      history.replace(getAuthRedirect(location));
    }
    return {
      fetchUserInfo,
      settings: defaultSettings as Partial<LayoutSettings>,
    };
  }

  const currentUser = await fetchUserInfo();
  if (!currentUser) {
    clearAuthState();
    history.replace(getAuthRedirect(location));
    return {
      fetchUserInfo,
      settings: defaultSettings as Partial<LayoutSettings>,
    };
  }

  // load language after auth is known so public first paint is not blocked by API calls
  let languageData;
  try {
    languageData = await withTimeout(() => getLanguages({ pageSize: 999 }), 2500);
  } catch (e) {}
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

  let appConfig, userConfig;
  try {
    appConfig = await withTimeout(() => getAppConfigsProfile({ skipErrorHandler: true }), 2500);
  } catch (e) {}
  try {
    userConfig = await withTimeout(() => getUserConfigsProfile({ skipErrorHandler: true }), 2500);
  } catch (e) {}
  //set title
  defaultSettings.title = appConfig?.base?.websiteName || 'mss-boot-admin';
  defaultSettings.logo = appConfig?.base?.websiteLogo || 'https://docs.mss-boot-io.top/favicon.ico';
  // set theme
  defaultSettings.navTheme =
    userConfig?.theme?.navTheme || appConfig?.theme?.navTheme || defaultSettings.navTheme;
  defaultSettings.layout =
    userConfig?.theme?.layout || appConfig?.theme?.layout || defaultSettings.layout;
  defaultSettings.contentWidth =
    userConfig?.theme?.contentWidth ||
    appConfig?.theme?.contentWidth ||
    defaultSettings.contentWidth;
  defaultSettings.fixedHeader =
    userConfig?.theme?.fixedHeader || appConfig?.theme?.fixedHeader || defaultSettings.fixedHeader;
  defaultSettings.fixSiderbar =
    userConfig?.theme?.fixSiderbar || appConfig?.theme?.fixSiderbar || defaultSettings.fixSiderbar;
  defaultSettings.colorWeak =
    userConfig?.theme?.colorWeak || appConfig?.theme?.colorWeak || defaultSettings.colorWeak;
  defaultSettings.pwa = userConfig?.theme?.pwa || appConfig?.theme?.pwa || defaultSettings.pwa;
  defaultSettings.colorPrimary =
    userConfig?.theme?.colorPrimary ||
    appConfig?.theme?.colorPrimary ||
    defaultSettings.colorPrimary;
  // defaultSettings.splitMenus = userConfig?.theme?.splitMenus || appConfig?.theme?.splitMenus || defaultSettings.splitMenus;

  if (token && !isLoginPage) {
    try {
      return {
        appConfig,
        userConfig,
        fetchUserInfo,
        currentUser,
        settings: defaultSettings as Partial<LayoutSettings>,
      };
    } catch (error) {
      clearAuthState();
      if (location.pathname !== loginPath) {
        history.replace(getAuthRedirect(location));
      }
      return {
        appConfig,
        userConfig,
        fetchUserInfo,
        settings: defaultSettings as Partial<LayoutSettings>,
      };
    }
  }

  return {
    appConfig,
    userConfig,
    fetchUserInfo,
    settings: defaultSettings as Partial<LayoutSettings>,
  };
}

// ProLayout 支持的api https://procomponents.ant.design/components/layout
export const layout: RunTimeLayoutConfig = ({ initialState, setInitialState }) => {
  return {
    title: initialState?.appConfig?.base?.websiteName || 'mss-boot-admin',
    menu: {
      locale: true,
      request: async () => {
        const menuData = await getMenuAuthorize();
        return menuData;
      },
    },
    actionsRender: () => [
      <HeaderSearch key="search" placeholder="component.search.placeholder" options={undefined} />,
      <NoticeIconView key="notice" />,
      <Question key="doc" />,
      <SelectLang key="SelectLang" />,
    ],
    avatarProps: {
      src: initialState?.currentUser?.avatar || undefined,
      title: <AvatarName />,
      render: (_, avatarChildren) => {
        return <AvatarDropdown menu={true}>{avatarChildren}</AvatarDropdown>;
      },
    },
    waterMarkProps: {
      content: initialState?.currentUser?.name,
    },
    footerRender: () => <Footer />,
    onPageChange: () => {
      const { location } = history;
      const token = localStorage.getItem('token');
      if (!initialState?.currentUser && !token && location.pathname !== loginPath) {
        history.replace(getAuthRedirect(location));
      }
    },
    layoutBgImgList: [
      {
        src: 'https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/D2LWSqNny4sAAAAAAAAAAAAAFl94AQBr',
        left: 85,
        bottom: 100,
        height: '303px',
      },
      {
        src: 'https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/C2TWRpJpiC0AAAAAAAAAAAAAFl94AQBr',
        bottom: -68,
        right: -45,
        height: '303px',
      },
      {
        src: 'https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/F6vSTbj8KpYAAAAAAAAAAAAAFl94AQBr',
        bottom: 0,
        left: 0,
        width: '331px',
      },
    ],
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
          <SettingDrawer
            disableUrlParams
            enableDarkTheme
            settings={initialState?.settings}
            onSettingChange={(settings) => {
              setInitialState((preInitialState) => ({
                ...preInitialState,
                settings,
              }));
            }}
          />
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
