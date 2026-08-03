// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 获取菜单下的接口 获取菜单下的接口 GET /admin/api/menu/api/${param0} */
export async function getMenuApiId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getMenuApiIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Menu[]>(`/admin/api/menu/api/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 获取菜单权限 获取菜单权限 GET /admin/api/menu/authorize */
export async function getMenuAuthorize(options?: { [key: string]: any }) {
  return request<(API.Menu & { children?: API.Menu[] })[]>('/admin/api/menu/authorize', {
    method: 'GET',
    ...(options || {}),
  });
}

/** 更新菜单权限 更新菜单权限 PUT /admin/api/menu/authorize/${param0} */
export async function putMenuAuthorizeId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putMenuAuthorizeIdParams,
  body: API.UpdateAuthorizeRequest,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/menu/authorize/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 绑定菜单下的接口 绑定菜单下的接口 POST /admin/api/menu/bind-api */
export async function postMenuBindApi(
  body: API.MenuBindAPIRequest,
  options?: { [key: string]: any },
) {
  return request<any>('/admin/api/menu/bind-api', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取菜单树 获取菜单树 GET /admin/api/menu/tree */
export async function getMenuTree(options?: { [key: string]: any }) {
  return request<(API.Menu & { children?: API.Menu[] })[]>('/admin/api/menu/tree', {
    method: 'GET',
    ...(options || {}),
  });
}

/** 菜单列表数据 菜单列表数据 GET /admin/api/menus */
export async function getMenus(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getMenusParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.Menu[] }>('/admin/api/menus', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建菜单 创建菜单 POST /admin/api/menus */
export async function postMenus(body: API.Menu, options?: { [key: string]: any }) {
  return request<API.Menu>('/admin/api/menus', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取菜单 获取菜单 GET /admin/api/menus/${param0} */
export async function getMenusId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getMenusIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Menu>(`/admin/api/menus/${param0}`, {
    method: 'GET',
    params: {
      ...queryParams,
    },
    ...(options || {}),
  });
}

/** 更新菜单 更新菜单 PUT /admin/api/menus/${param0} */
export async function putMenusId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putMenusIdParams,
  body: API.Menu,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Menu>(`/admin/api/menus/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除菜单 删除菜单 DELETE /admin/api/menus/${param0} */
export async function deleteMenusId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteMenusIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/menus/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
