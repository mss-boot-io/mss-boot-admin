// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 在线会话列表 GET /admin/api/online-sessions */
export async function getOnlineSessions(
  params: API.getOnlineSessionsParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.UserSession[] }>('/admin/api/online-sessions', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

/** 在线会话详情 GET /admin/api/online-sessions/${param0} */
export async function getOnlineSession(
  params: API.getOnlineSessionParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.UserSession>(`/admin/api/online-sessions/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 强制下线指定会话 DELETE /admin/api/online-sessions/${param0} */
export async function deleteOnlineSession(
  params: API.deleteOnlineSessionParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.OnlineSessionRevokeResult>(`/admin/api/online-sessions/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 强制下线该用户全部会话 DELETE /admin/api/online-sessions/user/${param0} */
export async function deleteOnlineSessionUser(
  params: API.deleteOnlineSessionUserParams,
  options?: { [key: string]: any },
) {
  const { userID: param0, ...queryParams } = params;
  return request<API.OnlineSessionRevokeUserResult>(`/admin/api/online-sessions/user/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 当前用户自登出 POST /admin/api/online-sessions/logout */
export async function postOnlineSessionLogout(options?: { [key: string]: any }) {
  return request<{ ok: boolean }>('/admin/api/online-sessions/logout', {
    method: 'POST',
    ...(options || {}),
  });
}
