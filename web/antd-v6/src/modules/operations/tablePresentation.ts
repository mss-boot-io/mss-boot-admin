import { corePresentationRegistry } from '../../generated/core-presentation-registry.generated';

export const taskPresentationRegistryEntry = corePresentationRegistry['task.list'];
export const taskPresentationListComponents = {
  name: 'text',
  provider: 'task-provider',
  spec: 'code',
  status: 'task-status',
  checkedAt: 'date-time',
  remark: 'text',
} as const;
export const taskPresentationSearchComponents = {
  name: 'input',
  status: 'status-filter',
} as const;
export const taskPresentationMobileFields = [
  'name',
  'provider',
  'spec',
  'status',
  'checkedAt',
] as const;

export const noticePresentationRegistryEntry = corePresentationRegistry['notice.list'];
export const noticePresentationListComponents = {
  title: 'notice-title',
  type: 'notice-type',
  status: 'notice-status',
  description: 'text',
  createdAt: 'date-time',
} as const;
export const noticePresentationSearchComponents = {
  title: 'input',
  type: 'select',
  status: 'select',
} as const;
export const noticePresentationMobileFields = [
  'title',
  'type',
  'status',
  'description',
  'createdAt',
] as const;

export const systemConfigPresentationRegistryEntry = corePresentationRegistry['system-config.list'];
export const systemConfigPresentationListComponents = {
  name: 'system-config-name',
  ext: 'system-config-format',
  remark: 'text',
  updatedAt: 'date-time',
} as const;
export const systemConfigPresentationSearchComponents = {} as const;
export const systemConfigPresentationMobileFields = ['name', 'ext', 'remark', 'updatedAt'] as const;

export const loginLogPresentationRegistryEntry = corePresentationRegistry['log.login'];
export const loginLogPresentationListComponents = {
  username: 'text',
  ip: 'text',
  location: 'text',
  status: 'status-tag',
  message: 'operational-message',
  loginAt: 'date-time',
} as const;
export const loginLogPresentationSearchComponents = { username: 'input' } as const;

export const auditLogPresentationRegistryEntry = corePresentationRegistry['log.audit'];
export const auditLogPresentationListComponents = {
  username: 'text',
  type: 'audit-log-type',
  action: 'text',
  message: 'operational-message',
  resource: 'text',
  path: 'code',
  status: 'status-tag',
  duration: 'duration',
  createdAt: 'date-time',
} as const;
export const auditLogPresentationSearchComponents = {
  username: 'input',
  type: 'select',
} as const;

export const runtimeLogPresentationRegistryEntry = corePresentationRegistry['log.runtime'];
export const runtimeLogPresentationListComponents = {
  timestamp: 'date-time',
  level: 'runtime-log-level',
  message: 'text',
} as const;
export const runtimeLogPresentationSearchComponents = {
  level: 'select',
  keyword: 'input',
  timeRange: 'date-time-range',
} as const;
