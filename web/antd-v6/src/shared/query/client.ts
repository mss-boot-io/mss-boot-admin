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
};
