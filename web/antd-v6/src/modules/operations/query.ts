import { queryKeys } from '@mss-admin-core/shared/query/client';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { operationsAPI } from './api';
import type {
  AuditLogType,
  NoticeListParams,
  OperationsListParams,
  OperationsPageSize,
  RuntimeLogParams,
} from './contract';

export function useTaskPage(params: OperationsListParams) {
  return useQuery({
    queryKey: queryKeys.taskList(params),
    queryFn: () => operationsAPI.tasks.list(params),
    placeholderData: keepPreviousData,
    staleTime: 15_000,
  });
}

export function useTask(id?: string) {
  return useQuery({
    queryKey: queryKeys.task(id ?? ''),
    queryFn: () => operationsAPI.tasks.get(id as string),
    enabled: Boolean(id),
    gcTime: 0,
    staleTime: 0,
  });
}

export function useTaskFunctions(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.taskFunctions,
    queryFn: operationsAPI.tasks.functions,
    enabled,
    staleTime: 60_000,
  });
}

export function useNoticePage(params: NoticeListParams) {
  return useQuery({
    queryKey: queryKeys.noticeList(params),
    queryFn: () => operationsAPI.notices.list(params),
    placeholderData: keepPreviousData,
    staleTime: 15_000,
  });
}

export function useNotice(id?: string) {
  return useQuery({
    queryKey: queryKeys.notice(id ?? ''),
    queryFn: () => operationsAPI.notices.get(id as string),
    enabled: Boolean(id),
    staleTime: 15_000,
  });
}

export function useLoginLogPage(params: { current: number; pageSize: number; username?: string }) {
  return useQuery({
    queryKey: queryKeys.loginLogs(params),
    queryFn: () => operationsAPI.logs.login(params),
    placeholderData: keepPreviousData,
    staleTime: 10_000,
  });
}

export function useAuditLogPage(params: {
  current: number;
  pageSize: number;
  username?: string;
  type?: AuditLogType | 'all';
}) {
  return useQuery({
    queryKey: queryKeys.auditLogs(params),
    queryFn: () => operationsAPI.logs.audit(params),
    placeholderData: keepPreviousData,
    staleTime: 10_000,
  });
}

export function useRuntimeLogPage(params: RuntimeLogParams, enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.runtimeLogs(params),
    queryFn: () => operationsAPI.logs.runtime(params),
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 5_000,
  });
}

export function useRuntimeLogFiles(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.runtimeLogFiles,
    queryFn: operationsAPI.logs.files,
    enabled,
    staleTime: 30_000,
  });
}

export function useSystemConfigPage(params: { current: number; pageSize: OperationsPageSize }) {
  return useQuery({
    queryKey: queryKeys.systemConfigList(params),
    queryFn: () => operationsAPI.systemConfigs.list(params),
    placeholderData: keepPreviousData,
    staleTime: 10_000,
  });
}

export function useSystemConfig(id?: string) {
  return useQuery({
    queryKey: queryKeys.systemConfig(id ?? ''),
    queryFn: () => operationsAPI.systemConfigs.get(id as string),
    enabled: Boolean(id),
    gcTime: 0,
    staleTime: 0,
  });
}
