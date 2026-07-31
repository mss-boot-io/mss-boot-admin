// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 用户配置分组 用户配置分组 GET /admin/api/user-configs/${param0} */
export async function getUserConfigsGroup(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getUserConfigsGroupParams,
  options?: { [key: string]: any },
) {
  const { group: param0, ...queryParams } = params;
  return request<Record<string, any>>(`/admin/api/user-configs/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 用户配置控制 用户配置控制 PUT /admin/api/user-configs/${param0} */
export async function putUserConfigsGroup(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putUserConfigsGroupParams,
  body: API.UserConfigControlRequest,
  options?: { [key: string]: any },
) {
  const { group: param0, ...queryParams } = params;
  return request<any>(`/admin/api/user-configs/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 用户配置 用户配置 GET /admin/api/user-configs/profile */
export async function getUserConfigsProfile(options?: { [key: string]: any }) {
  return request<Record<string, any>>('/admin/api/user-configs/profile', {
    method: 'GET',
    ...(options || {}),
  });
}
