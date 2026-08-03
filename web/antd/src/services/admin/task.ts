// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 操作任务 操作任务 GET /admin/api/task/${param1}/${param0} */
export async function getTaskOperateId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getTaskOperateIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, operate: param1, ...queryParams } = params;
  return request<any>(`/admin/api/task/${param1}/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 任务函数列表 任务函数列表 GET /admin/api/task/func-list */
export async function getTaskFuncList(options?: { [key: string]: any }) {
  return request<API.TaskFuncItem[]>('/admin/api/task/func-list', {
    method: 'GET',
    ...(options || {}),
  });
}

/** 任务列表 任务列表 GET /admin/api/tasks */
export async function getTasks(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getTasksParams,
  options?: { [key: string]: any },
) {
  return request<API.Page & { data?: API.Task[] }>('/admin/api/tasks', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 创建任务 创建任务 POST /admin/api/tasks */
export async function postTasks(body: API.Task, options?: { [key: string]: any }) {
  return request<API.Task>('/admin/api/tasks', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 获取任务 获取任务 GET /admin/api/tasks/${param0} */
export async function getTasksId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getTasksIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Task>(`/admin/api/tasks/${param0}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  });
}

/** 更新任务 更新任务 PUT /admin/api/tasks/${param0} */
export async function putTasksId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.putTasksIdParams,
  body: API.Task,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<API.Task>(`/admin/api/tasks/${param0}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    params: { ...queryParams },
    data: body,
    ...(options || {}),
  });
}

/** 删除任务 删除任务 DELETE /admin/api/tasks/${param0} */
export async function deleteTasksId(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteTasksIdParams,
  options?: { [key: string]: any },
) {
  const { id: param0, ...queryParams } = params;
  return request<any>(`/admin/api/tasks/${param0}`, {
    method: 'DELETE',
    params: { ...queryParams },
    ...(options || {}),
  });
}
