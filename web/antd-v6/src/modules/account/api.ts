import { request } from '@umijs/max';
import {
  type AccessTokenSecret,
  type AccessTokenSummary,
  type AccountSecurityStatus,
  type NotificationSettingKey,
  type NotificationSettings,
  type OAuthAuthorizeAttempt,
  type OAuthBinding,
  type OAuthIntent,
  type OAuthProvider,
  type ProfileUpdate,
  parseAccessTokenPage,
  parseAccessTokenSecret,
  parseAccountSecurityStatus,
  parseAvatarResponse,
  parseNotificationSettings,
  parseOAuthAuthorizeAttempt,
  parseOAuthBindings,
  parsePasswordChangeResponse,
  serializeNotificationSetting,
} from './contracts';

export interface AccountRequestOptions {
  method: 'GET' | 'POST' | 'PUT' | 'DELETE';
  data?: unknown;
  params?: Record<string, string>;
  skipErrorHandler: true;
}

export type AccountRequestClient = (
  path: string,
  options: AccountRequestOptions,
) => Promise<unknown>;

export interface AccountAPI {
  updateProfile: (profile: ProfileUpdate) => Promise<void>;
  uploadAvatar: (file: Blob) => Promise<string>;
  listAccessTokens: () => Promise<AccessTokenSummary[]>;
  createAccessToken: (validityPeriod: string) => Promise<AccessTokenSecret>;
  rotateAccessToken: (id: string) => Promise<AccessTokenSecret>;
  revokeAccessToken: (id: string) => Promise<void>;
  listOAuthBindings: () => Promise<OAuthBinding[]>;
  startOAuthAuthorization: (
    provider: OAuthProvider,
    intent: OAuthIntent,
  ) => Promise<OAuthAuthorizeAttempt>;
  loadSecurityStatus: () => Promise<AccountSecurityStatus>;
  reauthenticateWithPassword: (password: string) => Promise<AccountSecurityStatus>;
  changePassword: (newPassword: string) => Promise<{ signedOut: true }>;
  disconnectOAuth: (provider: OAuthProvider) => Promise<void>;
  loadNotificationSettings: () => Promise<NotificationSettings>;
  updateNotificationSetting: (key: NotificationSettingKey, enabled: boolean) => Promise<void>;
}

export function createAccountAPI(client: AccountRequestClient): AccountAPI {
  return {
    updateProfile: async (profile) => {
      await client('/user/userInfo', {
        method: 'PUT',
        data: profile,
        skipErrorHandler: true,
      });
    },
    uploadAvatar: async (file) => {
      const form = new FormData();
      form.append('file', file);
      return parseAvatarResponse(
        await client('/user/avatar', {
          method: 'POST',
          data: form,
          skipErrorHandler: true,
        }),
      );
    },
    listAccessTokens: async () =>
      parseAccessTokenPage(
        await client('/user-auth-tokens', { method: 'GET', skipErrorHandler: true }),
      ),
    createAccessToken: async (validityPeriod) =>
      parseAccessTokenSecret(
        await client('/user-auth-tokens', {
          method: 'POST',
          params: { validityPeriod },
          skipErrorHandler: true,
        }),
      ),
    rotateAccessToken: async (id) =>
      parseAccessTokenSecret(
        await client(`/user-auth-token/${encodeURIComponent(id)}/refresh`, {
          method: 'PUT',
          skipErrorHandler: true,
        }),
      ),
    revokeAccessToken: async (id) => {
      await client(`/user-auth-token/${encodeURIComponent(id)}/revoke`, {
        method: 'PUT',
        skipErrorHandler: true,
      });
    },
    listOAuthBindings: async () =>
      parseOAuthBindings(await client('/user/oauth2', { method: 'GET', skipErrorHandler: true })),
    startOAuthAuthorization: async (provider, intent) =>
      parseOAuthAuthorizeAttempt(
        await client('/user/session/oauth2/authorize', {
          method: 'POST',
          data: { provider, intent },
          skipErrorHandler: true,
        }),
      ),
    loadSecurityStatus: async () =>
      parseAccountSecurityStatus(
        await client('/user/security', { method: 'GET', skipErrorHandler: true }),
      ),
    reauthenticateWithPassword: async (password) =>
      parseAccountSecurityStatus(
        await client('/user/security/reauthenticate', {
          method: 'POST',
          data: { method: 'password', password },
          skipErrorHandler: true,
        }),
      ),
    changePassword: async (newPassword) =>
      parsePasswordChangeResponse(
        await client('/user/security/password', {
          method: 'PUT',
          data: { newPassword },
          skipErrorHandler: true,
        }),
      ),
    disconnectOAuth: async (provider) => {
      await client(`/user/oauth2/${encodeURIComponent(provider)}`, {
        method: 'DELETE',
        skipErrorHandler: true,
      });
    },
    loadNotificationSettings: async () =>
      parseNotificationSettings(
        await client('/user-configs/notification', {
          method: 'GET',
          skipErrorHandler: true,
        }),
      ),
    updateNotificationSetting: async (key, enabled) => {
      await client('/user-configs/notification', {
        method: 'PUT',
        data: serializeNotificationSetting(key, enabled),
        skipErrorHandler: true,
      });
    },
  };
}

export const accountAPI = createAccountAPI((path, options) => request<unknown>(path, options));
