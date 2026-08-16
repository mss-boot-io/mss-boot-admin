import { request } from '@umijs/max';
import {
  type OnlineSession,
  type OnlineSessionListParams,
  type OnlineSessionPage,
  parseOnlineSession,
  parseOnlineSessionPage,
  parseRevokeUserResult,
  type RevokeUserResult,
} from './contract';

export interface SessionRequestOptions {
  method: 'DELETE' | 'GET';
  params?: OnlineSessionListParams;
  skipErrorHandler: true;
}

export type SessionRequestClient = (
  path: string,
  options: SessionRequestOptions,
) => Promise<unknown>;

export function createSessionAPI(client: SessionRequestClient) {
  return {
    loadPage: async (params: OnlineSessionListParams): Promise<OnlineSessionPage> =>
      parseOnlineSessionPage(
        await client('/online-sessions', { method: 'GET', params, skipErrorHandler: true }),
        params,
      ),
    loadOne: async (id: string): Promise<OnlineSession> =>
      parseOnlineSession(
        await client(`/online-sessions/${encodeURIComponent(id)}`, {
          method: 'GET',
          skipErrorHandler: true,
        }),
      ),
    revokeOne: async (id: string): Promise<void> => {
      await client(`/online-sessions/${encodeURIComponent(id)}`, {
        method: 'DELETE',
        skipErrorHandler: true,
      });
    },
    revokeUser: async (userID: string): Promise<RevokeUserResult> =>
      parseRevokeUserResult(
        await client(`/online-sessions/user/${encodeURIComponent(userID)}`, {
          method: 'DELETE',
          skipErrorHandler: true,
        }),
        userID,
      ),
  };
}

export const sessionAPI = createSessionAPI((path, options) => request<unknown>(path, options));
