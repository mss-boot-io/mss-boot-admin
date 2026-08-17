import { request } from '@umijs/max';
import {
  type OptionDetail,
  type OptionFormValues,
  type OptionListParams,
  type OptionPage,
  type OptionSummary,
  optionRevisionETag,
  parseOptionDetail,
  parseOptionPage,
  serializeOptionWrite,
} from './contract';

export interface OptionRequestOptions {
  method: 'DELETE' | 'GET' | 'POST' | 'PUT';
  data?: unknown;
  headers?: Record<string, string>;
  params?: Record<string, number | string | undefined>;
  skipErrorHandler: true;
}

export type OptionRequestClient = (path: string, options: OptionRequestOptions) => Promise<unknown>;

export function createOptionAPI(client: OptionRequestClient) {
  return {
    loadPage: async (params: OptionListParams): Promise<OptionPage> =>
      parseOptionPage(
        await client('/options', {
          method: 'GET',
          params: {
            current: params.current,
            pageSize: params.pageSize,
            status: params.status === 'all' ? undefined : params.status,
            category: params.category,
            name: params.name,
          },
          skipErrorHandler: true,
        }),
        params,
      ),
    loadOne: async (id: string): Promise<OptionDetail> =>
      parseOptionDetail(
        await client(`/options/${encodeURIComponent(id)}`, {
          method: 'GET',
          skipErrorHandler: true,
        }),
      ),
    create: async (values: OptionFormValues): Promise<OptionDetail> =>
      parseOptionDetail(
        await client('/options', {
          method: 'POST',
          data: serializeOptionWrite(values),
          skipErrorHandler: true,
        }),
      ),
    update: async (
      id: string,
      values: OptionFormValues,
      base: OptionDetail,
    ): Promise<OptionDetail> =>
      parseOptionDetail(
        await client(`/options/${encodeURIComponent(id)}`, {
          method: 'PUT',
          data: serializeOptionWrite(values, { base }),
          headers: { 'If-Match': optionRevisionETag(base) },
          skipErrorHandler: true,
        }),
      ),
    remove: async (option: Pick<OptionSummary, 'id' | 'version'>): Promise<void> => {
      await client(`/options/${encodeURIComponent(option.id)}`, {
        method: 'DELETE',
        headers: { 'If-Match': optionRevisionETag(option) },
        skipErrorHandler: true,
      });
    },
  };
}

export const optionAPI = createOptionAPI((path, options) => request<unknown>(path, options));
