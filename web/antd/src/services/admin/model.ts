// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 生成数据 生成数据 PUT /admin/api/model/generate-data */
export async function putModelGenerateData(
  body: API.ModelGenerateDataRequest,
  options?: { [key: string]: any },
) {
  return request<any>('/admin/api/model/generate-data', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 模型列表 模型列表 GET /admin/api/models */
export async function getModels(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getModelsParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.Model[] }>('/admin/api/models', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建模型 创建模型 POST /admin/api/models */
export async function postModels(body: API.Model, options?: { [key: string]: any }) {
  return request<API.Model>('/admin/api/models', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取模型 获取模型 GET /admin/api/models/${param0} */
export async function getModelsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getModelsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Model>(`/admin/api/models/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 更新模型 PUT /admin/api/models/${param0} */
export async function putModelsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putModelsIdParams,
  body: API.Model,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Model>(`/admin/api/models/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除模型 删除模型 DELETE /admin/api/models/${param0} */
export async function deleteModelsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteModelsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/models/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
