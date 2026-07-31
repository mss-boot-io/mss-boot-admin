// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 文档 文档 GET /admin/api/documentation/${param0} */
export async function getDocumentationKey(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getDocumentationKeyParams,
  options?: { [key: string]: any },
) {
  const { key: param0, ...queryParams } = params;
  return request<API.VirtualModelObject>(`/admin/api/documentation/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}
