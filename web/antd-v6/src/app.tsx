import GithubOutlined from '@ant-design/icons/GithubOutlined';
import LogoutOutlined from '@ant-design/icons/LogoutOutlined';
import QuestionCircleOutlined from '@ant-design/icons/QuestionCircleOutlined';
import SettingOutlined from '@ant-design/icons/SettingOutlined';
import UserOutlined from '@ant-design/icons/UserOutlined';
import type { ProLayoutProps } from '@ant-design/pro-components';
import { QueryClientProvider } from '@tanstack/react-query';
import type { RequestConfig, RunTimeLayoutConfig } from '@umijs/max';
import { addLocale, history, Link, useIntl } from '@umijs/max';
import { App as AntdApp, Avatar, Button, Dropdown, Typography } from 'antd';
import type { ReactNode } from 'react';
import { languageAPI } from './modules/language/api';
import { registerSupportedLanguageProfile } from './modules/language/runtime';
import { getRequestStatus, requestConfig } from './shared/api/client';
import { RuntimeFeedbackBridge } from './shared/api/feedback';
import AuthorizationFreshnessBridge from './shared/auth/AuthorizationFreshnessBridge';
import { fetchAuthorizedMenu } from './shared/auth/authorization';
import SessionRefreshBridge from './shared/auth/SessionRefreshBridge';
import {
  clearServerSession,
  fetchCurrentUser,
  isPublicPath,
  redirectToLogin,
} from './shared/auth/session';
import type { InitialState, StartupFailure } from './shared/auth/types';
import { PageError } from './shared/design-system/PageState';
import { RuntimeMessageProvider } from './shared/i18n/runtime';
import HeaderNotice from './shared/navigation/HeaderNotice';
import LocaleSwitcher from './shared/navigation/LocaleSwitcher';
import MenuSearch from './shared/navigation/MenuSearch';
import { resolveMenuIcons } from './shared/navigation/menuIcons';
import { queryClient, queryKeys } from './shared/query/client';
import AuthorizationRealtimeBridge from './shared/realtime/AuthorizationRealtimeBridge';
import { loadApplicationProfile, loadThemeResource } from './shared/theme/api';
import { type ApplicationProfile, buildLayoutSettings } from './shared/theme/contract';
import {
  clearUserThemeRuntime,
  getThemeRuntimeSnapshot,
  replaceThemeRuntime,
} from './shared/theme/runtime';
import { ThemeCrossTabBridge } from './shared/theme/ThemeCrossTabBridge';
import { ThemeRuntimeProvider } from './shared/theme/ThemeRuntimeProvider';

interface Settled<T> {
  data?: T;
  error?: unknown;
}

async function settle<T>(promise: Promise<T>): Promise<Settled<T>> {
  try {
    return { data: await promise };
  } catch (error) {
    return { error };
  }
}

async function settleWithin<T>(promise: Promise<T>, timeoutMs: number): Promise<Settled<T>> {
  let timeout: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      settle(promise),
      new Promise<Settled<T>>((resolve) => {
        timeout = setTimeout(
          () => resolve({ error: new Error('Optional startup resource timed out') }),
          timeoutMs,
        );
      }),
    ]);
  } finally {
    if (timeout) clearTimeout(timeout);
  }
}

function applyLanguageProfile(
  profile: Settled<Awaited<ReturnType<typeof languageAPI.loadProfile>>>,
): void {
  if (profile.data) registerSupportedLanguageProfile(profile.data, addLocale);
}

async function loadVerifiedCurrentUser() {
  const value = await queryClient.fetchQuery({
    queryKey: queryKeys.currentUser,
    queryFn: async () => (await fetchCurrentUser()) ?? null,
    staleTime: 0,
  });
  return value ?? undefined;
}

function currentLayoutSettings(profile?: ApplicationProfile): Partial<ProLayoutProps> {
  return buildLayoutSettings(getThemeRuntimeSnapshot().resolved.settings, profile?.base);
}

export async function getInitialState(): Promise<InitialState> {
  const {
    clearThemeIdentitySession,
    ensureThemeAuthSession,
    readThemeBootstrapSnapshot,
    readThemeSnapshot,
    writeThemeSnapshot,
  } = await import('./shared/theme/snapshot');
  const publicRoute = isPublicPath(history.location.pathname);
  const bootstrap = readThemeBootstrapSnapshot();
  replaceThemeRuntime({
    application: bootstrap.application,
    degradedScopes: bootstrap.application ? ['application'] : [],
  });
  const applicationProfilePromise = settle(
    queryClient.fetchQuery({
      queryKey: queryKeys.applicationProfile,
      queryFn: loadApplicationProfile,
    }),
  );
  const languageProfilePromise = settleWithin(
    queryClient.fetchQuery({
      queryKey: queryKeys.languageProfile,
      queryFn: languageAPI.loadProfile,
      staleTime: 5 * 60_000,
    }),
    2_500,
  );

  if (publicRoute) {
    const [application, languageProfile] = await Promise.all([
      applicationProfilePromise,
      languageProfilePromise,
    ]);
    applyLanguageProfile(languageProfile);
    const applicationTheme = application.data?.theme ?? bootstrap.application;
    replaceThemeRuntime({
      application: applicationTheme,
      degradedScopes: application.error ? ['application'] : [],
    });
    if (application.data?.theme) {
      void writeThemeSnapshot(application.data.theme, undefined, Date.now(), {
        authoritativePrevious: bootstrap.application,
      });
    }
    return {
      applicationProfile: application.data,
      authorizationVersion: 0,
      authorizedMenu: [],
      fetchCurrentUser: loadVerifiedCurrentUser,
      settings: currentLayoutSettings(application.data),
      themeDegradedScopes: application.error ? ['application'] : undefined,
    };
  }

  const [application, identity, languageProfile] = await Promise.all([
    applicationProfilePromise,
    settle(loadVerifiedCurrentUser()),
    languageProfilePromise,
  ]);
  applyLanguageProfile(languageProfile);
  const applicationTheme = application.data?.theme ?? bootstrap.application;
  replaceThemeRuntime({
    application: applicationTheme,
    degradedScopes: application.error ? ['application'] : [],
  });
  if (application.data?.theme) {
    void writeThemeSnapshot(application.data.theme, undefined, Date.now(), {
      authoritativePrevious: bootstrap.application,
    });
  }

  if (identity.error) {
    return {
      applicationProfile: application.data,
      authorizationVersion: 0,
      authorizedMenu: [],
      fetchCurrentUser: loadVerifiedCurrentUser,
      settings: currentLayoutSettings(application.data),
      startupFailure: { area: 'identity', status: getRequestStatus(identity.error) },
      themeDegradedScopes: application.error ? ['application'] : undefined,
    };
  }

  const currentUser = identity.data;
  if (!currentUser) {
    clearThemeIdentitySession();
    redirectToLogin();
    return {
      applicationProfile: application.data,
      authorizationVersion: 0,
      authorizedMenu: [],
      fetchCurrentUser: loadVerifiedCurrentUser,
      settings: currentLayoutSettings(application.data),
      themeDegradedScopes: application.error ? ['application'] : undefined,
    };
  }

  const authSessionId = ensureThemeAuthSession(currentUser.id);
  const userBootstrap = readThemeSnapshot('user', authSessionId);
  replaceThemeRuntime({
    application: applicationTheme,
    user: userBootstrap,
    degradedScopes: [
      ...(application.error ? (['application'] as const) : []),
      ...(userBootstrap ? (['user'] as const) : []),
    ],
  });

  const [authorizedMenu, userTheme] = await Promise.all([
    settle(
      queryClient.fetchQuery({
        queryKey: queryKeys.authorizedMenu(currentUser.id),
        queryFn: () => fetchAuthorizedMenu(currentUser),
        staleTime: 0,
      }),
    ),
    settle(
      queryClient.fetchQuery({
        queryKey: queryKeys.theme('user', currentUser.id),
        queryFn: () => loadThemeResource('user'),
      }),
    ),
  ]);

  const degradedScopes = [
    ...(application.error ? (['application'] as const) : []),
    ...(userTheme.error ? (['user'] as const) : []),
  ];
  const userThemeResource = userTheme.data ?? userBootstrap;
  replaceThemeRuntime({
    application: applicationTheme,
    user: userThemeResource,
    degradedScopes: [...degradedScopes],
  });
  if (userTheme.data) {
    void writeThemeSnapshot(userTheme.data, authSessionId, Date.now(), {
      authoritativePrevious: userBootstrap,
    });
  }

  return {
    applicationProfile: application.data,
    authorizationVersion: 1,
    currentUser,
    authSessionId,
    authorizedMenu: authorizedMenu.data ?? [],
    fetchCurrentUser: loadVerifiedCurrentUser,
    settings: currentLayoutSettings(application.data),
    startupFailure: authorizedMenu.error
      ? { area: 'authorization', status: getRequestStatus(authorizedMenu.error) }
      : undefined,
    themeDegradedScopes: degradedScopes.length > 0 ? [...degradedScopes] : undefined,
  };
}

function StartupFailureView({ failure }: { failure: StartupFailure }) {
  const intl = useIntl();
  const messageID =
    failure.area === 'identity'
      ? 'startup.identityUnavailable'
      : 'startup.authorizationUnavailable';
  const status = failure.status ? ` (HTTP ${failure.status})` : '';
  return (
    <PageError
      message={`${intl.formatMessage({ id: messageID })}${status}`}
      onRetry={() => window.location.reload()}
      retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      title={intl.formatMessage({ id: 'states.loadError' })}
    />
  );
}

function AvatarMenu({ initialState }: { initialState?: InitialState }) {
  const intl = useIntl();
  const currentUser = initialState?.currentUser;
  const label = currentUser?.name ?? currentUser?.username ?? 'MSS User';
  return (
    <Dropdown
      menu={{
        items: [
          {
            key: 'center',
            icon: <UserOutlined />,
            label: (
              <Link to="/account/center">{intl.formatMessage({ id: 'menu.account-center' })}</Link>
            ),
          },
          {
            key: 'settings',
            icon: <SettingOutlined />,
            label: (
              <Link to="/account/settings">
                {intl.formatMessage({ id: 'menu.account-settings' })}
              </Link>
            ),
          },
          {
            key: 'logout',
            icon: <LogoutOutlined />,
            label: intl.formatMessage({ id: 'menu.logout' }),
            onClick: async () => {
              const { clearThemeIdentitySession } = await import('./shared/theme/snapshot');
              const previousSessionID = clearThemeIdentitySession({
                broadcast: false,
                expectedSessionId: initialState?.authSessionId,
              });
              queryClient.clear();
              clearUserThemeRuntime();
              try {
                await clearServerSession();
              } finally {
                if (previousSessionID) {
                  void import('./shared/theme/sync')
                    .then(({ publishThemeIdentityCleared }) =>
                      publishThemeIdentityCleared(previousSessionID),
                    )
                    .catch(() => {});
                }
                history.replace('/user/login');
              }
            },
          },
        ],
      }}
    >
      <span className="inline-flex cursor-pointer items-center gap-2">
        <Avatar src={currentUser?.avatar || undefined}>{label.slice(0, 1).toUpperCase()}</Avatar>
        <Typography.Text>{label}</Typography.Text>
      </span>
    </Dropdown>
  );
}

function DocumentationLink() {
  const intl = useIntl();
  return (
    <Button
      type="text"
      href="https://docs.mss-boot-io.top"
      target="_blank"
      rel="noreferrer"
      icon={<QuestionCircleOutlined />}
      aria-label={intl.formatMessage({ id: 'navigation.documentation' })}
    />
  );
}

function ApplicationFooter({ initialState }: { initialState?: InitialState }) {
  const base = initialState?.applicationProfile?.base;
  const copyright =
    typeof base?.websiteCopyRight === 'string' && base.websiteCopyRight.trim()
      ? base.websiteCopyRight.trim()
      : 'mss-boot-io';
  const recordNumber =
    typeof base?.websiteRecordNumber === 'string' ? base.websiteRecordNumber.trim() : '';

  return (
    <footer className="flex flex-col items-center gap-2 py-4 text-center text-sm text-neutral-500">
      <div>
        © {new Date().getFullYear()} {copyright}
      </div>
      <div className="flex flex-wrap items-center justify-center gap-x-4 gap-y-1">
        {recordNumber ? (
          <Typography.Link href="https://beian.miit.gov.cn" target="_blank" rel="noreferrer">
            {recordNumber}
          </Typography.Link>
        ) : null}
        <Typography.Link
          href="https://github.com/mss-boot-io/mss-boot"
          target="_blank"
          rel="noreferrer"
        >
          <GithubOutlined /> mss-boot
        </Typography.Link>
        <Typography.Link
          href="https://github.com/mss-boot-io/mss-boot-admin"
          target="_blank"
          rel="noreferrer"
        >
          mss-boot-admin
        </Typography.Link>
      </div>
    </footer>
  );
}

export const layout: RunTimeLayoutConfig = ({ initialState }) => ({
  ...initialState?.settings,
  actionsRender: () => [
    <MenuSearch key="menu-search" items={initialState?.authorizedMenu ?? []} />,
    <HeaderNotice key="notice" user={initialState?.currentUser} />,
    <DocumentationLink key="documentation" />,
    <LocaleSwitcher key="language" />,
  ],
  avatarProps: {
    render: () => <AvatarMenu initialState={initialState} />,
  },
  breadcrumbRender: (routers = []) => routers,
  childrenRender: (children) =>
    initialState?.startupFailure ? (
      <StartupFailureView failure={initialState.startupFailure} />
    ) : (
      children
    ),
  footerRender: () => <ApplicationFooter initialState={initialState} />,
  menu: {
    params: { authorizationVersion: initialState?.authorizationVersion ?? 0 },
    request: async () => initialState?.authorizedMenu ?? [],
  },
  menuDataRender: resolveMenuIcons,
  menuItemRender: (item, dom) =>
    item.path ? (
      <Link to={item.path} prefetch>
        {dom}
      </Link>
    ) : (
      dom
    ),
  onPageChange: () => {
    if (
      !initialState?.startupFailure &&
      !initialState?.currentUser &&
      !isPublicPath(history.location.pathname)
    ) {
      redirectToLogin();
    }
  },
  pageTitleRender: false,
  waterMarkProps: {
    content:
      initialState?.currentUser?.name?.trim() || initialState?.currentUser?.username?.trim() || '',
    gapX: 220,
    gapY: 180,
    font: { fontSize: 14 },
  },
});

export const request: RequestConfig = requestConfig;

function RuntimeProviders({ children }: { children: ReactNode }) {
  const intl = useIntl();
  return (
    <RuntimeMessageProvider formatMessage={(messageID) => intl.formatMessage({ id: messageID })}>
      <ThemeRuntimeProvider>
        <AntdApp>
          <RuntimeFeedbackBridge />
          <QueryClientProvider client={queryClient}>
            <SessionRefreshBridge />
            <AuthorizationFreshnessBridge />
            <AuthorizationRealtimeBridge />
            <ThemeCrossTabBridge />
            {children}
          </QueryClientProvider>
        </AntdApp>
      </ThemeRuntimeProvider>
    </RuntimeMessageProvider>
  );
}

// Umi applies innerProvider before wrapping the application in its dataflow
// provider. The resulting RuntimeProviders subtree is therefore inside the
// @@initialState model context when it renders. Do not move model-consuming
// bridges to rootContainer, which is intentionally the outermost runtime hook.
export function innerProvider(container: ReactNode) {
  return <RuntimeProviders>{container}</RuntimeProviders>;
}
