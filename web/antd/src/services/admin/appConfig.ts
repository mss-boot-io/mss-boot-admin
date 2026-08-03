// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 应用配置分组 应用配置分组 GET /admin/api/app-configs/${param0} */
export async function getAppConfigsGroup(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getAppConfigsGroupParams,
  options?: { [key: string]: any },
) {
  const { group: param0, ...queryParams } = params;
  return request<Record<string, any>>(`/admin/api/app-configs/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 应用配置控制 应用配置控制 PUT /admin/api/app-configs/${param0} */
export async function putAppConfigsGroup(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putAppConfigsGroupParams,
  body: API.AppConfigControlRequest,
  options?: { [key: string]: any },
) {
  const { group: param0, ...queryParams } = params;
  return request<any>(`/admin/api/app-configs/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 获取应用配置 获取应用配置 GET /admin/api/app-configs/profile */
export async function getAppConfigsProfile(options?: { [key: string]: any }) {
  return request<Record<string, any>>('/admin/api/app-configs/profile', {
    method: 'GET',
    ...(options || {}),
  });
}
