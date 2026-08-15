import { LogoutOutlined, SettingOutlined } from '@ant-design/icons';
import type { ProLayoutProps } from '@ant-design/pro-components';
import { QueryClientProvider } from '@tanstack/react-query';
import type { RequestConfig, RunTimeLayoutConfig } from '@umijs/max';
import { history, Link, useIntl } from '@umijs/max';
import { App as AntdApp, Avatar, Dropdown, Tag, Typography } from 'antd';
import type { ReactNode } from 'react';
import { getRequestStatus, requestConfig } from './shared/api/client';
import { RuntimeFeedbackBridge } from './shared/api/feedback';
import { fetchAuthorizedMenu } from './shared/auth/authorization';
import {
  clearServerSession,
  fetchCurrentUser,
  isPublicPath,
  redirectToLogin,
} from './shared/auth/session';
import type { InitialState, StartupFailure } from './shared/auth/types';
import { PageError } from './shared/design-system/PageState';
import { queryClient, queryKeys } from './shared/query/client';
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

  if (publicRoute) {
    const application = await applicationProfilePromise;
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
      authorizedMenu: [],
      fetchCurrentUser: loadVerifiedCurrentUser,
      settings: currentLayoutSettings(application.data),
      themeDegradedScopes: application.error ? ['application'] : undefined,
    };
  }

  const [application, identity] = await Promise.all([
    applicationProfilePromise,
    settle(loadVerifiedCurrentUser()),
  ]);
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
        <Avatar src={currentUser?.avatar}>{label.slice(0, 1).toUpperCase()}</Avatar>
        <Typography.Text>{label}</Typography.Text>
      </span>
    </Dropdown>
  );
}

export const layout: RunTimeLayoutConfig = ({ initialState }) => ({
  ...initialState?.settings,
  actionsRender: () => [
    <Tag key="antd" color="blue">
      antd {__ANTD_VERSION__}
    </Tag>,
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
  footerRender: () => (
    <div className="py-4 text-center text-sm text-neutral-500">
      MSS Admin v6 · <Link to="/migration">migration status</Link>
    </div>
  ),
  menu: {
    request: async () => initialState?.authorizedMenu ?? [],
  },
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
});

export const request: RequestConfig = requestConfig;

function RuntimeProviders({ children }: { children: ReactNode }) {
  return (
    <ThemeRuntimeProvider>
      <AntdApp>
        <RuntimeFeedbackBridge />
        <QueryClientProvider client={queryClient}>
          <ThemeCrossTabBridge />
          {children}
        </QueryClientProvider>
      </AntdApp>
    </ThemeRuntimeProvider>
  );
}

export function rootContainer(container: ReactNode) {
  return <RuntimeProviders>{container}</RuntimeProviders>;
}
