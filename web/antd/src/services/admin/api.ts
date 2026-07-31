// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** API列表数据 API列表数据 GET /admin/api/apis */
export async function getApis(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getApisParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.API[] }>('/admin/api/apis', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建API 创建API POST /admin/api/apis */
export async function postApis(body: API.API, options?: { [key: string]: any }) {
  return request<API.API>('/admin/api/apis', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取API 获取API GET /admin/api/apis/${param0} */
export async function getApisId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getApisIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.API>(`/admin/api/apis/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 更新API 更新API PUT /admin/api/apis/${param0} */
export async function putApisId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putApisIdParams,
  body: API.API,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.API>(`/admin/api/apis/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除API 删除API DELETE /admin/api/apis/${param0} */
export async function deleteApisId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteApisIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/apis/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
