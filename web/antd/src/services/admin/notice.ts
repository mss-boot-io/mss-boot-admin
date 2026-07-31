// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 获取通知 获取通知 GET /admin/api/notice/read/${param0} */
export async function getNoticeReadId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getNoticeReadIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Notice>(`/admin/api/notice/read/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 标记已读 标记已读 PUT /admin/api/notice/read/${param0} */
export async function putNoticeReadId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putNoticeReadIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/notice/read/${param0}`, {
    method: 'PUT',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 获取未读通知列表 获取未读通知列表 GET /admin/api/notice/unread */
export async function getNoticeUnread(options?: { [key: string]: any }) {
  return request<API.Notice[]>('/admin/api/notice/unread', {
    method: 'GET',
    ...(options || {}),
  });
}

/** 通知列表数据 通知列表数据 GET /admin/api/notices */
export async function getNotices(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getNoticesParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.Notice[] }>('/admin/api/notices', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建通知 创建通知 POST /admin/api/notices */
export async function postNotices(body: API.Notice, options?: { [key: string]: any }) {
  return request<any>('/admin/api/notices', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取通知 获取通知 GET /admin/api/notices/${param0} */
export async function getNoticesId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getNoticesIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Notice>(`/admin/api/notices/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 更新通知 更新通知 PUT /admin/api/notices/${param0} */
export async function putNoticesId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putNoticesIdParams,
  body: API.Notice,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/notices/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除通知 删除通知 DELETE /admin/api/notices/${param0} */
export async function deleteNoticesId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteNoticesIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/notices/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
