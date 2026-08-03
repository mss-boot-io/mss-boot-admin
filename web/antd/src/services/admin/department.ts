// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 部门列表 部门列表 GET /admin/api/departments */
export async function getDepartments(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getDepartmentsParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.Department[] }>('/admin/api/departments', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建部门 创建部门 POST /admin/api/departments */
export async function postDepartments(body: API.Department, options?: { [key: string]: any }) {
  return request<API.Department>('/admin/api/departments', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取部门 获取部门 GET /admin/api/departments/${param0} */
export async function getDepartmentsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getDepartmentsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Department>(`/admin/api/departments/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 更新部门 更新部门 PUT /admin/api/departments/${param0} */
export async function putDepartmentsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putDepartmentsIdParams,
  body: API.Department,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Department>(`/admin/api/departments/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除部门 删除部门 DELETE /admin/api/departments/${param0} */
export async function deleteDepartmentsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteDepartmentsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/departments/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
