import { LogoutOutlined } from '@ant-design/icons';
import type { ProLayoutProps } from '@ant-design/pro-components';
import { QueryClientProvider } from '@tanstack/react-query';
import type { RequestConfig, RunTimeLayoutConfig } from '@umijs/max';
import { history, Link, request as umiRequest } from '@umijs/max';
import { App as AntdApp, Avatar, ConfigProvider, Dropdown, Tag, Typography } from 'antd';
import type { ReactNode } from 'react';
import defaultSettings from '../config/defaultSettings';
import { requestConfig } from './shared/api/client';
import { RuntimeFeedbackBridge } from './shared/api/feedback';
import {
  clearServerSession,
  fetchCurrentUser,
  isPublicPath,
  redirectToLogin,
} from './shared/auth/session';
import type { AuthorizedMenuItem, InitialState } from './shared/auth/types';
import { createThemeConfig, defaultThemeSettings } from './shared/design-system/theme';
import { queryClient } from './shared/query/client';
import { retainRegisteredMenu } from './shared/routes/registry';

async function fetchAuthorizedMenu(): Promise<AuthorizedMenuItem[]> {
  try {
    const menu = await umiRequest<AuthorizedMenuItem[]>('/menu/authorize', {
      method: 'GET',
      skipErrorHandler: true,
    });
    return retainRegisteredMenu(menu ?? []);
  } catch {
    return [];
  }
}

export async function getInitialState(): Promise<InitialState> {
  const publicRoute = isPublicPath(history.location.pathname);
  const currentUser = publicRoute ? undefined : await fetchCurrentUser();
  if (!publicRoute && !currentUser) redirectToLogin();
  const authorizedMenu = currentUser ? await fetchAuthorizedMenu() : [];
  return {
    currentUser,
    authorizedMenu,
    fetchCurrentUser,
    settings: defaultSettings as Partial<ProLayoutProps>,
  };
}

function AvatarMenu({ currentUser }: { currentUser?: InitialState['currentUser'] }) {
  const label = currentUser?.name ?? currentUser?.username ?? 'MSS User';
  return (
    <Dropdown
      menu={{
        items: [
          {
            key: 'logout',
            icon: <LogoutOutlined />,
            label: '退出登录',
            onClick: async () => {
              await clearServerSession();
              queryClient.clear();
              history.replace('/user/login');
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
  actionsRender: () => [
    <Tag key="antd" color="blue">
      antd {__ANTD_VERSION__}
    </Tag>,
  ],
  avatarProps: {
    render: () => <AvatarMenu currentUser={initialState?.currentUser} />,
  },
  breadcrumbRender: (routers = []) => routers,
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
    if (!initialState?.currentUser && !isPublicPath(history.location.pathname)) {
      redirectToLogin();
    }
  },
  pageTitleRender: false,
  ...initialState?.settings,
});

export const request: RequestConfig = requestConfig;

function RuntimeProviders({ children }: { children: ReactNode }) {
  return (
    <ConfigProvider theme={createThemeConfig(defaultThemeSettings)}>
      <AntdApp>
        <RuntimeFeedbackBridge />
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </AntdApp>
    </ConfigProvider>
  );
}

export function rootContainer(container: ReactNode) {
  return <RuntimeProviders>{container}</RuntimeProviders>;
}
