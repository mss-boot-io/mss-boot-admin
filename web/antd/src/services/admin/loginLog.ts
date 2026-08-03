import { request } from '@umijs/max';

export async function getLoginLogs(params: API.LoginLogSearchParams) {
  return request<{ data: API.LoginLog[]; total: number; current: number; pageSize: number }>(
    '/admin/api/audit-logs/login',
    {
      method: 'GET',
      params,
    },
  );
}
