// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 获取语言配置 获取语言配置 GET /admin/api/language/profile */
export async function getLanguageProfile(options?: { [key: string]: any }) {
  return request<Record<string, any>>('/admin/api/language/profile', {
    method: 'GET',
    ...(options || {}),
  });
}

/** Language列表数据 Language列表数据 GET /admin/api/languages */
export async function getLanguages(
  params: API.getLanguagesParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.Language[] }>('/admin/api/languages/public', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建Language 创建Language POST /admin/api/languages */
export async function postLanguages(body: API.Language, options?: { [key: string]: any }) {
  return request<API.Language>('/admin/api/languages', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取Language 获取Language GET /admin/api/languages/${param0} */
export async function getLanguagesId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getLanguagesIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Language>(`/admin/api/languages/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 更新Language 更新Language PUT /admin/api/languages/${param0} */
export async function putLanguagesId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putLanguagesIdParams,
  body: API.Language,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Language>(`/admin/api/languages/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除Language 删除Language DELETE /admin/api/languages/${param0} */
export async function deleteLanguagesId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteLanguagesIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/languages/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
