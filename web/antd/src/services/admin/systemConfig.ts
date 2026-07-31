// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 系统配置列表数据 系统配置列表数据 GET /admin/api/system-configs */
export async function getSystemConfigs(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getSystemConfigsParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.SystemConfig[] }>('/admin/api/system-configs', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建系统配置 创建系统配置 POST /admin/api/system-configs */
export async function postSystemConfigs(body: API.SystemConfig, options?: { [key: string]: any }) {
  return request<API.SystemConfig>('/admin/api/system-configs', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取系统配置 获取系统配置 GET /admin/api/system-configs/${param0} */
export async function getSystemConfigsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getSystemConfigsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.SystemConfig>(`/admin/api/system-configs/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 更新系统配置 更新系统配置 PUT /admin/api/system-configs/${param0} */
export async function putSystemConfigsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putSystemConfigsIdParams,
  body: API.SystemConfig,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.SystemConfig>(`/admin/api/system-configs/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除系统配置 删除系统配置 DELETE /admin/api/system-configs/${param0} */
export async function deleteSystemConfigsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteSystemConfigsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/system-configs/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
