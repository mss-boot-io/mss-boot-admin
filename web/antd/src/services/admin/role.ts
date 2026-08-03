// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 获取角色授权 获取角色授权 GET /admin/api/role/authorize/${param0} */
export async function getRoleAuthorizeRoleId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getRoleAuthorizeRoleIDParams,
  options?: { [key: string]: any },
) {
  const { roleID: param0, ...queryParams } = params;
  return request<API.GetAuthorizeResponse>(`/admin/api/role/authorize/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 角色授权 给角色授权 POST /admin/api/role/authorize/${param0} */
export async function postRoleAuthorizeRoleId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.postRoleAuthorizeRoleIDParams,
  body: API.SetAuthorizeRequest,
  options?: { [key: string]: any },
) {
  const { roleID: param0, ...queryParams } = params;
  return request<any>(`/admin/api/role/authorize/${param0}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 角色列表 角色列表 GET /admin/api/roles */
export async function getRoles(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getRolesParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.Role[] }>('/admin/api/roles', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建角色 创建角色 POST /admin/api/roles */
export async function postRoles(body: API.Role, options?: { [key: string]: any }) {
  return request<API.Role>('/admin/api/roles', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取角色 获取角色 GET /admin/api/roles/${param0} */
export async function getRolesId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getRolesIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Role>(`/admin/api/roles/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 更新角色 更新角色 PUT /admin/api/roles/${param0} */
export async function putRolesId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putRolesIdParams,
  body: API.Role,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Role>(`/admin/api/roles/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除角色 删除角色 DELETE /admin/api/roles/${param0} */
export async function deleteRolesId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteRolesIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/roles/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
