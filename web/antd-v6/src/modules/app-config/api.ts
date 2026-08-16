import { request } from '@umijs/max';
import {
  type AppConfigGroup,
  parseBaseAppConfig,
  parseEmailAppConfig,
  parseSecurityAppConfig,
  parseStorageAppConfig,
  parseStorageUpload,
} from './contracts';

interface AppConfigRequestOptions {
  method: 'GET' | 'PUT' | 'POST';
  data?: unknown;
  skipErrorHandler: true;
}

export type AppConfigRequestClient = (
  path: string,
  options: AppConfigRequestOptions,
) => Promise<unknown>;

export function createAppConfigAPI(client: AppConfigRequestClient) {
  return {
    loadBase: async () =>
      parseBaseAppConfig(
        await client('/app-configs/base', { method: 'GET', skipErrorHandler: true }),
      ),
    loadSecurity: async () =>
      parseSecurityAppConfig(
        await client('/app-configs/security', { method: 'GET', skipErrorHandler: true }),
      ),
    loadStorage: async () =>
      parseStorageAppConfig(
        await client('/app-configs/storage', { method: 'GET', skipErrorHandler: true }),
      ),
    loadEmail: async () =>
      parseEmailAppConfig(
        await client('/app-configs/email', { method: 'GET', skipErrorHandler: true }),
      ),
    saveGroup: async (group: AppConfigGroup, data: Record<string, unknown>) => {
      await client(`/app-configs/${group}`, {
        method: 'PUT',
        data: { data },
        skipErrorHandler: true,
      });
    },
    uploadLogo: async (file: Blob) => {
      const form = new FormData();
      form.append('file', file);
      return parseStorageUpload(
        await client('/storage/upload', {
          method: 'POST',
          data: form,
          skipErrorHandler: true,
        }),
      );
    },
  };
}

export const appConfigAPI = createAppConfigAPI((path, options) => request<unknown>(path, options));
