import { QueryClient } from '@tanstack/react-query';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: (failureCount, error) => {
        const status = (error as { response?: { status?: number } }).response?.status;
        if (status && status >= 400 && status < 500) return false;
        return failureCount < 2;
      },
      staleTime: 30_000,
    },
    mutations: {
      retry: false,
    },
  },
});

export const queryKeys = {
  currentUser: ['identity', 'current-user'] as const,
  authorizedMenu: (userID: string) => ['authorization', 'menu', userID] as const,
  applicationProfile: ['configuration', 'application-profile'] as const,
  theme: (scope: 'application' | 'user', owner = '') =>
    ['configuration', 'theme', scope, owner] as const,
  accountTokens: (userID: string) => ['account', userID, 'access-tokens'] as const,
  accountOAuth: (userID: string) => ['account', userID, 'oauth-bindings'] as const,
  accountNotifications: (userID: string) => ['account', userID, 'notifications'] as const,
  workplace: ['workplace'] as const,
  monitor: ['operations', 'monitor'] as const,
  onlineSessions: ['security', 'online-sessions'] as const,
  onlineSessionList: (
    params: Readonly<{
      current: number;
      pageSize: number;
      status: string;
      userID?: string;
      username?: string;
      ip?: string;
    }>,
  ) => ['security', 'online-sessions', 'list', params] as const,
  onlineSession: (id: string) => ['security', 'online-sessions', 'detail', id] as const,
  languages: ['configuration', 'languages'] as const,
  languageProfile: ['configuration', 'language-profile'] as const,
  languageList: (
    params: Readonly<{
      current: number;
      pageSize: number;
      status: string;
      name?: string;
    }>,
  ) => ['configuration', 'languages', 'list', params] as const,
  language: (id: string) => ['configuration', 'languages', 'detail', id] as const,
  options: ['configuration', 'options'] as const,
  optionList: (
    params: Readonly<{
      current: number;
      pageSize: number;
      status: string;
      category?: string;
      name?: string;
    }>,
  ) => ['configuration', 'options', 'list', params] as const,
  option: (id: string) => ['configuration', 'options', 'detail', id] as const,
  administration: (resource: 'departments' | 'menus' | 'posts' | 'roles' | 'users') =>
    ['administration', resource] as const,
  administrationList: (
    resource: 'departments' | 'menus' | 'posts' | 'roles' | 'users',
    params: Readonly<{
      current: number;
      pageSize: number;
      name?: string;
      status?: string;
    }>,
  ) => ['administration', resource, 'list', params] as const,
  administrationDetail: (
    resource: 'departments' | 'menus' | 'posts' | 'roles' | 'users',
    id: string,
  ) => ['administration', resource, 'detail', id] as const,
  administrationTree: (resource: 'departments' | 'menus' | 'posts') =>
    ['administration', resource, 'tree'] as const,
  roleAuthorization: (roleID: string) =>
    ['administration', 'roles', roleID, 'authorization'] as const,
  tasks: ['operations', 'tasks'] as const,
  taskList: (params: Readonly<object>) => ['operations', 'tasks', 'list', params] as const,
  task: (id: string) => ['operations', 'tasks', 'detail', id] as const,
  taskFunctions: ['operations', 'tasks', 'functions'] as const,
  notices: ['operations', 'notices'] as const,
  noticeList: (params: Readonly<object>) => ['operations', 'notices', 'list', params] as const,
  notice: (id: string) => ['operations', 'notices', 'detail', id] as const,
  loginLogs: (params: Readonly<object>) => ['operations', 'logs', 'login', params] as const,
  auditLogs: (params: Readonly<object>) => ['operations', 'logs', 'audit', params] as const,
  runtimeLogs: (params: Readonly<object>) => ['operations', 'logs', 'runtime', params] as const,
  runtimeLogFiles: ['operations', 'logs', 'runtime', 'files'] as const,
  systemConfigs: ['operations', 'system-configs'] as const,
  systemConfigList: (params: Readonly<object>) =>
    ['operations', 'system-configs', 'list', params] as const,
  systemConfig: (id: string) => ['operations', 'system-configs', 'detail', id] as const,
};
