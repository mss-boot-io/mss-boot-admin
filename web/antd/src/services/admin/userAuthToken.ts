// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 刷新用户令牌 PUT /admin/api/user-auth-token/${param0}/refresh */
export async function putUserAuthTokenIdRefresh(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putUserAuthTokenIdRefreshParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.UserAuthToken>(`/admin/api/user-auth-token/${param0}/refresh`, {
    method: 'PUT',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 撤销用户令牌 PUT /admin/api/user-auth-token/${param0}/revoke */
export async function putUserAuthTokenIdRevoke(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putUserAuthTokenIdRevokeParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/user-auth-token/${param0}/revoke`, {
    method: 'PUT',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 生成用户令牌 GET /admin/api/user-auth-token/generate */
export async function getUserAuthTokenGenerate(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getUserAuthTokenGenerateParams,
  options?: { [key: string]: any },
) {
  return request<API.UserAuthToken>('/admin/api/user-auth-token/generate', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 列表 GET /admin/api/user-auth-tokens */
export async function getUserAuthTokens(options?: { [key: string]: any }) {
  return request<API.Page & { data?: API.UserAuthToken[] }>('/admin/api/user-auth-tokens', {
    method: 'GET',
    ...(options || {}),
  });
}
