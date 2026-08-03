// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 获取github登录地址 获取github登录地址 GET /admin/api/github/get-login-url */
export async function getGithubGetLoginUrl(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getGithubGetLoginUrlParams,
  options?: { [key: string]: any },
) {
  return request<string>('/admin/api/github/get-login-url', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 从模版生成代码 从模版生成代码 POST /admin/api/template/generate */
export async function postTemplateGenerate(
  body: API.TemplateGenerateReq,
  options?: { [key: string]: any },
) {
  return request<API.TemplateGenerateResp>('/admin/api/template/generate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取template分支 获取template分支 GET /admin/api/template/get-branches */
export async function getTemplateGetBranches(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getTemplateGetBranchesParams,
  options?: { [key: string]: any },
) {
  return request<API.TemplateGetBranchesResp>('/admin/api/template/get-branches', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 获取template参数配置 获取template参数配置 GET /admin/api/template/get-params */
export async function getTemplateGetParams(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getTemplateGetParamsParams,
  options?: { [key: string]: any },
) {
  return request<API.TemplateGetParamsResp>('/admin/api/template/get-params', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 获取template文件路径list 获取template文件路径list GET /admin/api/template/get-path */
export async function getTemplateGetPath(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getTemplateGetPathParams,
  options?: { [key: string]: any },
) {
  return request<API.TemplateGetPathResp>('/admin/api/template/get-path', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}
