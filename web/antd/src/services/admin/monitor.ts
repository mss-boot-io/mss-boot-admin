// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 获取监控信息 获取监控信息 GET /admin/api/monitor */
export async function getMonitor(options?: { [key: string]: any }) {
  return request<API.MonitorResponse>('/admin/api/monitor', {
    method: 'GET',
    ...(options || {}),
  });
}
