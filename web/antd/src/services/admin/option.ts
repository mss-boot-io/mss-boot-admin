// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** Option列表数据 Option列表数据 GET /admin/api/options */
export async function getOptions(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getOptionsParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.Option[] }>('/admin/api/options', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建Option 创建Option POST /admin/api/options */
export async function postOptions(body: API.Option, options?: { [key: string]: any }) {
  return request<API.Option>('/admin/api/options', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取Option 获取Option GET /admin/api/options/${param0} */
export async function getOptionsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getOptionsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Option>(`/admin/api/options/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 更新Option 更新Option PUT /admin/api/options/${param0} */
export async function putOptionsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putOptionsIdParams,
  body: API.Option,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Option>(`/admin/api/options/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除Option 删除Option DELETE /admin/api/options/${param0} */
export async function deleteOptionsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteOptionsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/options/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
