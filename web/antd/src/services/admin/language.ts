// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';
import {
  cacheLanguages,
  getCachedLanguages as getCachedLanguageData,
  invalidateLanguageCache,
} from './languageCache';

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

/**
 * Returns the public language bundle from a short-lived browser cache when it
 * is fresh. The public endpoint remains the source of truth after expiry.
 */
export async function getCachedLanguages(): Promise<API.Page & { data?: API.Language[] }> {
  const cachedLanguages = getCachedLanguageData();
  if (cachedLanguages !== undefined) {
    return { data: cachedLanguages };
  }

  const response = await getLanguages({ pageSize: 999 });
  // The public endpoint intentionally returns a raw array, while some
  // deployments wrap it in `{ data }`. Normalize both shapes at this
  // boundary so startup registration and the browser cache stay reliable.
  const languages = Array.isArray(response) ? response : response?.data;
  if (Array.isArray(languages)) {
    cacheLanguages(languages);
    return { data: languages };
  }

  return response;
}

/** 创建Language 创建Language POST /admin/api/languages */
export async function postLanguages(body: API.Language, options?: { [key: string]: any }) {
  const response = await request<API.Language>('/admin/api/languages', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
  invalidateLanguageCache();
  return response;
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
  const response = await request<API.Language>(`/admin/api/languages/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
  invalidateLanguageCache();
  return response;
}

/** 删除Language 删除Language DELETE /admin/api/languages/${param0} */
export async function deleteLanguagesId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteLanguagesIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  const response = await request<any>(`/admin/api/languages/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
  invalidateLanguageCache();
  return response;
}
