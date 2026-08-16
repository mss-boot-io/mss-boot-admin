import { request } from '@umijs/max';
import {
  type AuditLogEntry,
  type AuditLogType,
  type LoginLogEntry,
  type NoticeListParams,
  type NoticeSummary,
  type OperationsListParams,
  type OperationsPage,
  parseAuditLog,
  parseLoginLog,
  parseNotice,
  parseOperationsPage,
  parseRuntimeLogFiles,
  parseRuntimeLogPage,
  parseSystemConfigDetail,
  parseSystemConfigSummary,
  parseTaskDetail,
  parseTaskFunctions,
  parseTaskSummary,
  type RuntimeLogFiles,
  type RuntimeLogPage,
  type RuntimeLogParams,
  type SystemConfigDetail,
  type SystemConfigSummary,
  type SystemConfigWriteValues,
  serializeSystemConfigWrite,
  serializeTaskWrite,
  type TaskDetail,
  type TaskSummary,
  type TaskWriteValues,
} from './contract';

interface OperationsRequestOptions {
  method: 'DELETE' | 'GET' | 'POST' | 'PUT';
  data?: unknown;
  params?: Record<string, number | string | undefined>;
  skipErrorHandler: true;
}

export type OperationsRequestClient = (
  path: string,
  options: OperationsRequestOptions,
) => Promise<unknown>;

function entityPath(resource: string, id: string): string {
  return `/${resource}/${encodeURIComponent(id)}`;
}

function listParams(params: OperationsListParams) {
  return {
    current: params.current,
    pageSize: params.pageSize,
    name: params.name?.trim() || undefined,
    status: !params.status || params.status === 'all' ? undefined : params.status,
  };
}

export function runtimeLogExportPath(params: Omit<RuntimeLogParams, 'page' | 'pageSize'>): string {
  const query = new URLSearchParams();
  if (params.level) query.set('level', params.level);
  if (params.keyword?.trim()) query.set('keyword', params.keyword.trim());
  if (params.startTime) query.set('startTime', params.startTime);
  if (params.endTime) query.set('endTime', params.endTime);
  const encoded = query.toString();
  return `/admin/api/logs/export${encoded ? `?${encoded}` : ''}`;
}

export function createOperationsAPI(client: OperationsRequestClient) {
  return {
    tasks: {
      list: async (params: OperationsListParams): Promise<OperationsPage<TaskSummary>> =>
        parseOperationsPage(
          await client('/tasks', {
            method: 'GET',
            params: listParams(params),
            skipErrorHandler: true,
          }),
          params,
          parseTaskSummary,
        ),
      get: async (id: string): Promise<TaskDetail> =>
        parseTaskDetail(
          await client(entityPath('tasks', id), { method: 'GET', skipErrorHandler: true }),
        ),
      create: async (values: TaskWriteValues): Promise<TaskDetail> =>
        parseTaskDetail(
          await client('/tasks', {
            method: 'POST',
            data: serializeTaskWrite(values),
            skipErrorHandler: true,
          }),
        ),
      update: async (id: string, values: TaskWriteValues): Promise<TaskDetail> =>
        parseTaskDetail(
          await client(entityPath('tasks', id), {
            method: 'PUT',
            data: serializeTaskWrite(values),
            skipErrorHandler: true,
          }),
        ),
      remove: async (id: string): Promise<void> => {
        await client(entityPath('tasks', id), { method: 'DELETE', skipErrorHandler: true });
      },
      operate: async (id: string, operation: 'start' | 'stop'): Promise<void> => {
        await client(`/tasks/${encodeURIComponent(id)}/actions/${operation}`, {
          method: 'POST',
          skipErrorHandler: true,
        });
      },
      functions: async (): Promise<string[]> =>
        parseTaskFunctions(
          await client('/task/func-list', { method: 'GET', skipErrorHandler: true }),
        ),
    },
    notices: {
      unread: async (): Promise<NoticeSummary[]> => {
        const value = await client('/notice/unread', {
          method: 'GET',
          skipErrorHandler: true,
        });
        if (value === null || value === undefined) return [];
        if (!Array.isArray(value) || value.length > 100) {
          throw new Error('Unread notice list is invalid');
        }
        return value.map(parseNotice);
      },
      list: async (params: NoticeListParams): Promise<OperationsPage<NoticeSummary>> =>
        parseOperationsPage(
          await client('/notices', {
            method: 'GET',
            params: {
              current: params.current,
              pageSize: params.pageSize,
              title: params.title?.trim() || undefined,
              status: !params.status || params.status === 'all' ? undefined : params.status,
              type: !params.type || params.type === 'all' ? undefined : params.type,
            },
            skipErrorHandler: true,
          }),
          params,
          parseNotice,
        ),
      get: async (id: string): Promise<NoticeSummary> =>
        parseNotice(
          await client(`/notice/read/${encodeURIComponent(id)}`, {
            method: 'GET',
            skipErrorHandler: true,
          }),
        ),
      markRead: async (id: string): Promise<void> => {
        await client(`/notice/read/${encodeURIComponent(id)}`, {
          method: 'PUT',
          skipErrorHandler: true,
        });
      },
    },
    logs: {
      login: async (params: {
        current: number;
        pageSize: number;
        username?: string;
      }): Promise<OperationsPage<LoginLogEntry>> =>
        parseOperationsPage(
          await client('/audit-logs/login', {
            method: 'GET',
            params: {
              current: params.current,
              pageSize: params.pageSize,
              username: params.username?.trim() || undefined,
            },
            skipErrorHandler: true,
          }),
          params,
          parseLoginLog,
        ),
      audit: async (params: {
        current: number;
        pageSize: number;
        username?: string;
        type?: AuditLogType | 'all';
      }): Promise<OperationsPage<AuditLogEntry>> =>
        parseOperationsPage(
          await client('/audit-logs/operation', {
            method: 'GET',
            params: {
              current: params.current,
              pageSize: params.pageSize,
              username: params.username?.trim() || undefined,
              type: !params.type || params.type === 'all' ? undefined : params.type,
            },
            skipErrorHandler: true,
          }),
          params,
          parseAuditLog,
        ),
      runtime: async (params: RuntimeLogParams): Promise<RuntimeLogPage> =>
        parseRuntimeLogPage(
          await client('/logs', {
            method: 'GET',
            params: {
              page: params.page,
              pageSize: params.pageSize,
              level: params.level || undefined,
              keyword: params.keyword?.trim() || undefined,
              startTime: params.startTime,
              endTime: params.endTime,
            },
            skipErrorHandler: true,
          }),
        ),
      files: async (): Promise<RuntimeLogFiles> =>
        parseRuntimeLogFiles(
          await client('/logs/files', { method: 'GET', skipErrorHandler: true }),
        ),
    },
    systemConfigs: {
      list: async (
        params: Pick<OperationsListParams, 'current' | 'pageSize'>,
      ): Promise<OperationsPage<SystemConfigSummary>> =>
        parseOperationsPage(
          await client('/system-configs', {
            method: 'GET',
            params,
            skipErrorHandler: true,
          }),
          params,
          parseSystemConfigSummary,
        ),
      get: async (id: string): Promise<SystemConfigDetail> =>
        parseSystemConfigDetail(
          await client(entityPath('system-configs', id), {
            method: 'GET',
            skipErrorHandler: true,
          }),
        ),
      create: async (values: SystemConfigWriteValues): Promise<SystemConfigDetail> =>
        parseSystemConfigDetail(
          await client('/system-configs', {
            method: 'POST',
            data: serializeSystemConfigWrite(values),
            skipErrorHandler: true,
          }),
        ),
      update: async (id: string, values: SystemConfigWriteValues): Promise<SystemConfigDetail> =>
        parseSystemConfigDetail(
          await client(entityPath('system-configs', id), {
            method: 'PUT',
            data: serializeSystemConfigWrite(values),
            skipErrorHandler: true,
          }),
        ),
      remove: async (id: string): Promise<void> => {
        await client(entityPath('system-configs', id), {
          method: 'DELETE',
          skipErrorHandler: true,
        });
      },
    },
  };
}

export const operationsAPI = createOperationsAPI((path, options) =>
  request<unknown>(path, options),
);
