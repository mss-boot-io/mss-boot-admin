// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 字段列表 字段列表 GET /admin/api/fields */
export async function getFields(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getFieldsParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.Field[] }>('/admin/api/fields', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建字段 创建字段 POST /admin/api/fields */
export async function postFields(body: API.Field, options?: { [key: string]: any }) {
  return request<API.Field>('/admin/api/fields', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取字段 获取字段 GET /admin/api/fields/${param0} */
export async function getFieldsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getFieldsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Field>(`/admin/api/fields/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 更新字段 更新字段 PUT /admin/api/fields/${param0} */
export async function putFieldsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putFieldsIdParams,
  body: API.Field,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Field>(`/admin/api/fields/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除字段 删除字段 DELETE /admin/api/fields/${param0} */
export async function deleteFieldsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteFieldsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/fields/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
