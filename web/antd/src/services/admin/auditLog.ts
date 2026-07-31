import { request } from '@umijs/max';

export async function getAuditLogs(params: API.AuditLogSearchParams) {
  return request<{ data: API.AuditLog[]; total: number; current: number; pageSize: number }>(
    '/admin/api/audit-logs/operation',
    {
      method: 'GET',
      params,
    },
  );
}
