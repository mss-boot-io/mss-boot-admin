import { request } from '@umijs/max';
import { MONITOR_HISTORY_LIMIT, type MonitorSnapshot, parseMonitorSnapshot } from './contract';

export interface MonitorRequestOptions {
  method: 'GET';
  params: { historyLimit: number };
  skipErrorHandler: true;
}

export type MonitorRequestClient = (
  path: string,
  options: MonitorRequestOptions,
) => Promise<unknown>;

export function createMonitorAPI(client: MonitorRequestClient) {
  return {
    loadSnapshot: async (historyLimit = MONITOR_HISTORY_LIMIT): Promise<MonitorSnapshot> =>
      parseMonitorSnapshot(
        await client('/monitor', {
          method: 'GET',
          params: { historyLimit },
          skipErrorHandler: true,
        }),
      ),
  };
}

export const monitorAPI = createMonitorAPI((path, options) => request<unknown>(path, options));
