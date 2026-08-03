// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 获取统计 获取统计 GET /admin/api/statistics/${param0} */
export async function getStatisticsName(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getStatisticsNameParams,
  options?: { [key: string]: any },
) {
  const { name: param0, ...queryParams } = params;
  return request<API.StatisticsGetResponse>(`/admin/api/statistics/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}
