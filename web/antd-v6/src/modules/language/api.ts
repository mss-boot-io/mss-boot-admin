import { request } from '@umijs/max';
import {
  type LanguageDetail,
  type LanguageFormValues,
  type LanguageListParams,
  type LanguagePage,
  type LanguageProfile,
  parseLanguageDetail,
  parseLanguagePage,
  parseLanguageProfile,
  serializeLanguageWrite,
} from './contract';

export interface LanguageRequestOptions {
  method: 'DELETE' | 'GET' | 'POST' | 'PUT';
  data?: unknown;
  params?: Record<string, number | string | undefined>;
  skipErrorHandler: true;
}

export type LanguageRequestClient = (
  path: string,
  options: LanguageRequestOptions,
) => Promise<unknown>;

export function createLanguageAPI(client: LanguageRequestClient) {
  return {
    loadProfile: async (): Promise<LanguageProfile> =>
      parseLanguageProfile(
        await client('/language/profile', { method: 'GET', skipErrorHandler: true }),
      ),
    loadPage: async (params: LanguageListParams): Promise<LanguagePage> =>
      parseLanguagePage(
        await client('/languages', {
          method: 'GET',
          params: {
            current: params.current,
            pageSize: params.pageSize,
            status: params.status === 'all' ? undefined : params.status,
            name: params.name,
            view: 'summary',
          },
          skipErrorHandler: true,
        }),
        params,
      ),
    loadOne: async (id: string): Promise<LanguageDetail> =>
      parseLanguageDetail(
        await client(`/languages/${encodeURIComponent(id)}`, {
          method: 'GET',
          skipErrorHandler: true,
        }),
      ),
    create: async (values: LanguageFormValues): Promise<LanguageDetail> =>
      parseLanguageDetail(
        await client('/languages', {
          method: 'POST',
          data: serializeLanguageWrite(values),
          skipErrorHandler: true,
        }),
      ),
    update: async (
      id: string,
      values: LanguageFormValues,
      expectedUpdatedAt: string,
    ): Promise<LanguageDetail> =>
      parseLanguageDetail(
        await client(`/languages/${encodeURIComponent(id)}`, {
          method: 'PUT',
          data: serializeLanguageWrite(values, expectedUpdatedAt),
          skipErrorHandler: true,
        }),
      ),
    remove: async (id: string): Promise<void> => {
      await client(`/languages/${encodeURIComponent(id)}`, {
        method: 'DELETE',
        skipErrorHandler: true,
      });
    },
  };
}

export const languageAPI = createLanguageAPI((path, options) => request<unknown>(path, options));
