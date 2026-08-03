// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 岗位列表 岗位列表 GET /admin/api/posts */
export async function getPosts(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getPostsParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.Post[] }>('/admin/api/posts', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建岗位 创建岗位 POST /admin/api/posts */
export async function postPosts(body: API.Post, options?: { [key: string]: any }) {
  return request<API.Post>('/admin/api/posts', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取岗位 获取岗位 GET /admin/api/posts/${param0} */
export async function getPostsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getPostsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Post>(`/admin/api/posts/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 更新岗位 更新岗位 PUT /admin/api/posts/${param0} */
export async function putPostsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putPostsIdParams,
  body: API.Post,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Post>(`/admin/api/posts/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除岗位 删除岗位 DELETE /admin/api/posts/${param0} */
export async function deletePostsId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deletePostsIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/posts/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
