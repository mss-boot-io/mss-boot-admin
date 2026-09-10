import { request } from '@umijs/max';
import type {
  BillsPage,
  BillsParams,
  KeyPageParams,
  KeySnapshot,
  Page,
  RechargeRecord,
  RechargeRequest,
  RemoteOrg,
  RemoteTeam,
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
  recharge: (id: string, body: RechargeRequest): Promise<RechargeRecord> =>
    request(`${base}/users/${encodeURIComponent(id)}/recharge`, {
      method: 'POST',
      data: body,
      skipErrorHandler: true,
    }),
  listOrgs: (): Promise<{ items: RemoteOrg[]; total: number }> =>
    request(`${base}/organizations`, { method: 'GET', skipErrorHandler: true }),
  getOrg: (id: string): Promise<RemoteOrg> =>
    request(`${base}/organizations/${encodeURIComponent(id)}`, {
      method: 'GET',
      skipErrorHandler: true,
    }),
  createOrg: (body: {
    organization_alias: string;
    max_budget?: number;
    models?: string[];
  }): Promise<RemoteOrg> =>
    request(`${base}/organizations`, { method: 'POST', data: body, skipErrorHandler: true }),
  updateOrg: (
    id: string,
    body: { organization_alias?: string; max_budget?: number; models?: string[] },
  ): Promise<RemoteOrg> =>
    request(`${base}/organizations/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      data: body,
      skipErrorHandler: true,
    }),
  addOrgMember: (id: string, body: { user_email: string; role: string }): Promise<RemoteOrg> =>
    request(`${base}/organizations/${encodeURIComponent(id)}/members`, {
      method: 'POST',
      data: body,
      skipErrorHandler: true,
    }),
  removeOrgMember: (
    id: string,
    body: { user_id?: string; user_email?: string },
  ): Promise<RemoteOrg> =>
    request(`${base}/organizations/${encodeURIComponent(id)}/members/remove`, {
      method: 'POST',
      data: body,
      skipErrorHandler: true,
    }),
  createTeam: (
    orgId: string,
    body: { team_alias: string; max_budget?: number; models?: string[] },
  ): Promise<RemoteTeam> =>
    request(`${base}/organizations/${encodeURIComponent(orgId)}/teams`, {
      method: 'POST',
      data: body,
      skipErrorHandler: true,
    }),
  deleteTeam: (teamId: string) =>
    request(`${base}/teams/${encodeURIComponent(teamId)}`, {
      method: 'DELETE',
      skipErrorHandler: true,
    }),
  deleteOrg: (id: string) =>
    request(`${base}/organizations/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      skipErrorHandler: true,
    }),
};
