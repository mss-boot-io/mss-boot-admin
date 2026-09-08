import { request } from '@umijs/max';
import type {
  BillsPage,
  BillsParams,
  KeyPageParams,
  KeySnapshot,
  Page,
  SyncReport,
  UserDetail,
  UserPageParams,
  UserSnapshot,
} from './types';

const base = '/litellmops';

export const litellmopsAPI = {
  listUsers: (params: UserPageParams): Promise<Page<UserSnapshot>> =>
    request(`${base}/users`, { method: 'GET', params, skipErrorHandler: true }),
  getUser: (id: string): Promise<UserDetail> =>
    request(`${base}/users/${encodeURIComponent(id)}`, {
      method: 'GET',
      skipErrorHandler: true,
    }),
  listKeys: (params: KeyPageParams): Promise<Page<KeySnapshot>> =>
    request(`${base}/keys`, { method: 'GET', params, skipErrorHandler: true }),
  sync: (): Promise<SyncReport> =>
    request(`${base}/sync`, { method: 'POST', skipErrorHandler: true }),
  listBills: (params: BillsParams): Promise<BillsPage> =>
    request(`${base}/bills`, { method: 'GET', params, skipErrorHandler: true }),
};
