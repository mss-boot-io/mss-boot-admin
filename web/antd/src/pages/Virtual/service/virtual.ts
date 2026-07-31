// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

export async function listVirtualModels(
  // @ts-ignore
  params: API.listVirtualModelsParams,
  options?: { [key: string]: any },
) {
  const { key: param0, ...queryParams } = params;
  return request<API.Page & { data?: { [key: string]: any }[] }>(`/admin/api/${param0}`, {
    method: 'GET',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  });
}

export async function getVirtualModel(
  // @ts-ignore
  params: API.getVirtualModelParams,
  options?: { [key: string]: any },
) {
  const { key, id, ...queryParams } = params;
  return request<{ [key: string]: any }>(`/admin/api/${key}/${id}`, {
    method: 'GET',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  });
}

export async function createVirtualModel(
  // @ts-ignore
  params: API.createVirtualModelParams,
  body: { [key: string]: any },
  options?: { [key: string]: any },
) {
  const { key, ...queryParams } = params;
  return request<{ [key: string]: any }>(`/admin/api/${key}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: {
      ...queryParams,
    },
    data: body,
    ...(options || {}),
  });
}

export async function updateVirtualModel(
  // @ts-ignore
  params: API.updateVirtualModelParams,
  body: { [key: string]: any },
  options?: { [key: string]: any },
) {
  const { key, id, ...queryParams } = params;
  return request<{ [key: string]: any }>(`/admin/api/${key}/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: {
      ...queryParams,
    },
    data: body,
    ...(options || {}),
  });
}

export async function deleteVirtualModel(
  // @ts-ignore
  params: API.deleteVirtualModelParams,
  options?: { [key: string]: any },
) {
  const { key, id, ...queryParams } = params;
  return request<any>(`/admin/api/${key}/${id}`, {
    method: 'DELETE',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  });
}

export async function getVirtualDocumentation(
  // @ts-ignore
  params: API.getVirtualDocumentationParams,
  options?: { [key: string]: any },
) {
  const { key, ...queryParams } = params;
  // @ts-ignore
  return request<API.documentation>(`/admin/api/documentation/${key}`, {
    method: 'GET',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  });
}
