import { request } from '@umijs/max';

export async function getRuntimeLogs(params: API.LogSearchParams) {
  return request<API.LogListResponse>('/admin/api/logs', {
    method: 'GET',
    params,
  });
}

export async function exportRuntimeLogs(params?: { level?: string; keyword?: string }) {
  const query = new URLSearchParams();
  if (params?.level) query.append('level', params.level);
  if (params?.keyword) query.append('keyword', params.keyword);
  return `/admin/api/logs/export?${query.toString()}`;
}
