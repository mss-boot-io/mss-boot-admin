export const OPERATIONS_PAGE_SIZES = [20, 50, 100] as const;
export const MAX_OPERATIONS_PAGE_SIZE = 100;
export const MAX_SYSTEM_CONFIG_BYTES = 256 * 1024;

export type OperationsPageSize = (typeof OPERATIONS_PAGE_SIZES)[number];
export type OperationalStatus = '' | 'disabled' | 'enabled' | 'locked';
export type OperationalStatusFilter = OperationalStatus | 'all';

export interface OperationsListParams {
  current: number;
  pageSize: OperationsPageSize;
  name?: string;
  status?: OperationalStatusFilter;
}

export interface OperationsPage<T> {
  data: T[];
  total: number;
  current: number;
  pageSize: number;
}

export type TaskProvider = 'default' | 'func' | 'k8s';

export interface TaskSummary {
  id: string;
  createdAt: string;
  updatedAt: string;
  name: string;
  provider: TaskProvider;
  spec: string;
  status: 'disabled' | 'enabled';
  checkedAt?: string;
  remark: string;
}

export interface TaskDetail extends TaskSummary {
  namespace: string;
  cluster: string;
  image: string;
  command: string[];
  args: string[];
  protocol: '' | 'http' | 'https';
  endpoint: string;
  body: string;
  timeout: number;
  method: string;
  metadata: string;
}

export interface TaskWriteValues {
  name: string;
  provider: TaskProvider;
  spec: string;
  remark?: string;
  timeout?: number;
  protocol?: 'http' | 'https';
  endpoint?: string;
  method?: string;
  body?: string;
  metadata?: string;
  cluster?: string;
  namespace?: string;
  image?: string;
  command?: string[];
  args?: string[];
}

export type NoticeType = 'event' | 'mail' | 'message' | 'notification';
export type NoticeStatus = '' | 'doing' | 'processing' | 'todo' | 'urgent';

export interface NoticeListParams {
  current: number;
  pageSize: OperationsPageSize;
  title?: string;
  status?: NoticeStatus | 'all';
  type?: NoticeType | 'all';
}

export interface NoticeSummary {
  id: string;
  title: string;
  key: string;
  read: boolean;
  avatar: string;
  extra: string;
  status: NoticeStatus;
  description: string;
  datetime?: string;
  type: NoticeType;
  createdAt: string;
  updatedAt: string;
}

export type AuditLogType =
  | 'config'
  | 'create'
  | 'delete'
  | 'export'
  | 'import'
  | 'login'
  | 'logout'
  | 'security'
  | 'update';

export interface AuditLogEntry {
  id: string;
  userID: string;
  username: string;
  type: AuditLogType;
  action: string;
  resource: string;
  method: string;
  path: string;
  ip: string;
  userAgent: string;
  status: OperationalStatus;
  message: string;
  duration: number;
  createdAt: string;
}

export interface LoginLogEntry {
  id: string;
  userID: string;
  username: string;
  ip: string;
  location: string;
  userAgent: string;
  status: OperationalStatus;
  message: string;
  loginAt: string;
  logoutAt?: string;
}

export type RuntimeLogLevel = '' | 'debug' | 'error' | 'fatal' | 'info' | 'trace' | 'warn';

export interface RuntimeLogParams {
  page: number;
  pageSize: OperationsPageSize;
  level?: RuntimeLogLevel;
  keyword?: string;
  startTime?: string;
  endTime?: string;
}

export interface RuntimeLogEntry {
  timestamp: string;
  level: string;
  message: string;
  raw: string;
}

export interface RuntimeLogPage {
  list: RuntimeLogEntry[];
  total: number;
  truncated: boolean;
}

export interface RuntimeLogFiles {
  files: string[];
  truncated: boolean;
}

export type SystemConfigFormat = 'json' | 'yaml' | 'yml';

export interface SystemConfigSummary {
  id: string;
  createdAt: string;
  updatedAt: string;
  name: string;
  ext: SystemConfigFormat;
  remark: string;
  builtIn: boolean;
}

export interface SystemConfigDetail extends SystemConfigSummary {
  content: string;
}

export interface SystemConfigWriteValues {
  name: string;
  ext: SystemConfigFormat;
  content: string;
  remark?: string;
}

export class OperationsContractError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'OperationsContractError';
  }
}

function record(value: unknown, label: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new OperationsContractError(`${label} must be an object`);
  }
  return value as Record<string, unknown>;
}

function text(value: Record<string, unknown>, key: string, max: number, required = false): string {
  const candidate = value[key];
  if (candidate === undefined || candidate === null) {
    if (required) throw new OperationsContractError(`${key} is required`);
    return '';
  }
  if (typeof candidate !== 'string') {
    throw new OperationsContractError(`${key} must be a string`);
  }
  const normalized = candidate.trim();
  if ((required && !normalized) || [...normalized].length > max) {
    throw new OperationsContractError(`${key} is invalid`);
  }
  return normalized;
}

function untrimmedText(value: Record<string, unknown>, key: string, maxBytes: number): string {
  const candidate = value[key];
  if (candidate === undefined || candidate === null) return '';
  if (typeof candidate !== 'string' || new TextEncoder().encode(candidate).length > maxBytes) {
    throw new OperationsContractError(`${key} is invalid`);
  }
  return candidate;
}

function identifier(value: Record<string, unknown>, key = 'id'): string {
  return text(value, key, 64, true);
}

function bool(value: Record<string, unknown>, key: string): boolean {
  const candidate = value[key];
  if (typeof candidate !== 'boolean') {
    throw new OperationsContractError(`${key} must be a boolean`);
  }
  return candidate;
}

function integer(
  value: Record<string, unknown>,
  key: string,
  fallback?: number,
  max = 10_000_000,
): number {
  const candidate = value[key];
  if (candidate === undefined || candidate === null) {
    if (fallback !== undefined) return fallback;
    throw new OperationsContractError(`${key} is required`);
  }
  if (
    typeof candidate !== 'number' ||
    !Number.isSafeInteger(candidate) ||
    candidate < 0 ||
    candidate > max
  ) {
    throw new OperationsContractError(`${key} must be a bounded integer`);
  }
  return candidate;
}

function date(value: Record<string, unknown>, key: string, required = true): string | undefined {
  const candidate = text(value, key, 64, required);
  if (!candidate) return undefined;
  if (!Number.isFinite(Date.parse(candidate))) {
    throw new OperationsContractError(`${key} must be a date`);
  }
  return candidate;
}

function status(value: Record<string, unknown>): OperationalStatus {
  const candidate = text(value, 'status', 20);
  if (!['', 'disabled', 'enabled', 'locked'].includes(candidate)) {
    throw new OperationsContractError('status is invalid');
  }
  return candidate as OperationalStatus;
}

function parseStringArray(value: Record<string, unknown>, key: string): string[] {
  let candidate = value[key];
  if (candidate === undefined || candidate === null || candidate === '') return [];
  if (typeof candidate === 'string') {
    if (candidate.length > 16 * 1024) throw new OperationsContractError(`${key} is too large`);
    try {
      candidate = JSON.parse(candidate) as unknown;
    } catch {
      throw new OperationsContractError(`${key} must contain a JSON string array`);
    }
  }
  if (!Array.isArray(candidate) || candidate.length > 32) {
    throw new OperationsContractError(`${key} must be a bounded string array`);
  }
  return candidate.map((item) => {
    if (typeof item !== 'string' || !item.trim() || [...item.trim()].length > 1_024) {
      throw new OperationsContractError(`${key} contains an invalid item`);
    }
    return item.trim();
  });
}

export function isOperationsPageSize(value: number): value is OperationsPageSize {
  return OPERATIONS_PAGE_SIZES.includes(value as OperationsPageSize);
}

export function parseOperationsPage<T>(
  value: unknown,
  params: { current: number; pageSize: number },
  parseItem: (item: unknown) => T,
): OperationsPage<T> {
  const page = record(value, 'page');
  if (!Array.isArray(page.data) || page.data.length > MAX_OPERATIONS_PAGE_SIZE) {
    throw new OperationsContractError('page data is invalid');
  }
  const total = integer(page, 'total', 0);
  const current = integer(page, 'current', params.current, 10_000) || params.current;
  const pageSize =
    integer(page, 'pageSize', params.pageSize, MAX_OPERATIONS_PAGE_SIZE) || params.pageSize;
  if (current < 1 || pageSize < 1 || pageSize > MAX_OPERATIONS_PAGE_SIZE) {
    throw new OperationsContractError('page metadata is invalid');
  }
  return { data: page.data.map(parseItem), total, current, pageSize };
}

export function parseTaskSummary(value: unknown): TaskSummary {
  const task = record(value, 'task');
  const provider = text(task, 'provider', 20, true);
  if (!['default', 'func', 'k8s'].includes(provider)) {
    throw new OperationsContractError('task provider is invalid');
  }
  const taskStatus = status(task);
  if (taskStatus !== 'enabled' && taskStatus !== 'disabled') {
    throw new OperationsContractError('task status is invalid');
  }
  return {
    id: identifier(task),
    createdAt: date(task, 'createdAt') as string,
    updatedAt: date(task, 'updatedAt') as string,
    name: text(task, 'name', 255, true),
    provider: provider as TaskProvider,
    spec: text(task, 'spec', 255, true),
    status: taskStatus,
    checkedAt: date(task, 'checkedAt', false),
    remark: text(task, 'remark', 4_096),
  };
}

export function parseTaskDetail(value: unknown): TaskDetail {
  const task = record(value, 'task');
  const summary = parseTaskSummary(task);
  const protocol = text(task, 'protocol', 10);
  if (!['', 'http', 'https'].includes(protocol)) {
    throw new OperationsContractError('task protocol is invalid');
  }
  return {
    ...summary,
    namespace: text(task, 'namespace', 63),
    cluster: text(task, 'cluster', 50),
    image: text(task, 'image', 255),
    command: parseStringArray(task, 'command'),
    args: parseStringArray(task, 'args'),
    protocol: protocol as TaskDetail['protocol'],
    endpoint: text(task, 'endpoint', 255),
    body: untrimmedText(task, 'body', 64 * 1024),
    timeout: integer(task, 'timeout', 30, 3_600),
    method: text(task, 'method', 128),
    metadata: untrimmedText(task, 'metadata', 16 * 1024),
  };
}

export function serializeTaskWrite(values: TaskWriteValues): Record<string, unknown> {
  const name = values.name.trim();
  const spec = values.spec.trim();
  const remark = values.remark?.trim() ?? '';
  const timeout = values.timeout ?? 30;
  if (
    !name ||
    [...name].length > 255 ||
    !spec ||
    [...spec].length > 255 ||
    timeout < 1 ||
    timeout > 3_600
  ) {
    throw new OperationsContractError('task values are invalid');
  }
  const base: Record<string, unknown> = {
    name,
    provider: values.provider,
    spec,
    remark,
    timeout,
  };
  if (values.provider === 'default') {
    return {
      ...base,
      protocol: values.protocol,
      endpoint: values.endpoint?.trim(),
      method: values.method?.trim().toUpperCase(),
      body: values.body ?? '',
      metadata: values.metadata?.trim() ?? '',
    };
  }
  if (values.provider === 'func') {
    return {
      ...base,
      method: values.method?.trim(),
      args: JSON.stringify(values.args ?? []),
    };
  }
  return {
    ...base,
    cluster: values.cluster?.trim(),
    namespace: values.namespace?.trim() || 'default',
    image: values.image?.trim(),
    command: JSON.stringify(values.command ?? []),
    args: JSON.stringify(values.args ?? []),
  };
}

export function taskFormValues(task: TaskDetail): TaskWriteValues {
  return {
    name: task.name,
    provider: task.provider,
    spec: task.spec,
    remark: task.remark,
    timeout: task.timeout,
    protocol: task.protocol || undefined,
    endpoint: task.endpoint,
    method: task.method,
    body: task.body,
    metadata: task.metadata,
    cluster: task.cluster,
    namespace: task.namespace,
    image: task.image,
    command: task.command,
    args: task.args,
  };
}

export function parseTaskFunctions(value: unknown): string[] {
  if (!Array.isArray(value) || value.length > 100) {
    throw new OperationsContractError('task functions are invalid');
  }
  return value.map((item) => text(record(item, 'task function'), 'name', 128, true));
}

export function parseNotice(value: unknown): NoticeSummary {
  const notice = record(value, 'notice');
  const type = text(notice, 'type', 20, true);
  if (!['event', 'mail', 'message', 'notification'].includes(type)) {
    throw new OperationsContractError('notice type is invalid');
  }
  const noticeStatus = text(notice, 'status', 10);
  if (!['', 'doing', 'processing', 'todo', 'urgent'].includes(noticeStatus)) {
    throw new OperationsContractError('notice status is invalid');
  }
  return {
    id: identifier(notice),
    title: text(notice, 'title', 255, true),
    key: text(notice, 'key', 255),
    read: bool(notice, 'read'),
    avatar: text(notice, 'avatar', 255),
    extra: text(notice, 'extra', 255),
    status: noticeStatus as NoticeStatus,
    description: untrimmedText(notice, 'description', 16 * 1024),
    datetime: date(notice, 'datetime', false),
    type: type as NoticeType,
    createdAt: date(notice, 'createdAt') as string,
    updatedAt: date(notice, 'updatedAt') as string,
  };
}

export function parseAuditLog(value: unknown): AuditLogEntry {
  const log = record(value, 'audit log');
  const type = text(log, 'type', 50, true);
  if (
    ![
      'config',
      'create',
      'delete',
      'export',
      'import',
      'login',
      'logout',
      'security',
      'update',
    ].includes(type)
  ) {
    throw new OperationsContractError('audit type is invalid');
  }
  return {
    id: identifier(log),
    userID: text(log, 'userID', 64),
    username: text(log, 'username', 255),
    type: type as AuditLogType,
    action: text(log, 'action', 255),
    resource: text(log, 'resource', 255),
    method: text(log, 'method', 10),
    path: text(log, 'path', 500),
    ip: text(log, 'ip', 50),
    userAgent: text(log, 'userAgent', 500),
    status: status(log),
    message: untrimmedText(log, 'message', 4_096),
    duration: integer(log, 'duration', 0),
    createdAt: date(log, 'createdAt') as string,
  };
}

export function parseLoginLog(value: unknown): LoginLogEntry {
  const log = record(value, 'login log');
  return {
    id: identifier(log),
    userID: text(log, 'userID', 64),
    username: text(log, 'username', 255),
    ip: text(log, 'ip', 50),
    location: text(log, 'location', 255),
    userAgent: text(log, 'userAgent', 500),
    status: status(log),
    message: text(log, 'message', 500),
    loginAt: date(log, 'loginAt') as string,
    logoutAt: date(log, 'logoutAt', false),
  };
}

export function parseRuntimeLogPage(value: unknown): RuntimeLogPage {
  const page = record(value, 'runtime log page');
  if (!Array.isArray(page.list) || page.list.length > MAX_OPERATIONS_PAGE_SIZE) {
    throw new OperationsContractError('runtime log list is invalid');
  }
  return {
    list: page.list.map((item) => {
      const log = record(item, 'runtime log');
      return {
        timestamp: text(log, 'timestamp', 64),
        level: text(log, 'level', 20),
        message: untrimmedText(log, 'message', 16 * 1024),
        raw: untrimmedText(log, 'raw', 16 * 1024 + 3),
      };
    }),
    total: integer(page, 'total', 0, 10_000),
    truncated: page.truncated === undefined ? false : bool(page, 'truncated'),
  };
}

export function parseRuntimeLogFiles(value: unknown): RuntimeLogFiles {
  const result = record(value, 'runtime log files');
  if (!Array.isArray(result.files) || result.files.length > 32) {
    throw new OperationsContractError('runtime log files are invalid');
  }
  return {
    files: result.files.map((file) => {
      if (typeof file !== 'string' || !file || file.length > 255 || /[/\\]/.test(file)) {
        throw new OperationsContractError('runtime log filename is invalid');
      }
      return file;
    }),
    truncated: result.truncated === undefined ? false : bool(result, 'truncated'),
  };
}

export function parseSystemConfigSummary(value: unknown): SystemConfigSummary {
  const config = record(value, 'system configuration');
  const ext = text(config, 'ext', 16, true);
  if (!['json', 'yaml', 'yml'].includes(ext)) {
    throw new OperationsContractError('system configuration format is invalid');
  }
  return {
    id: identifier(config),
    createdAt: date(config, 'createdAt') as string,
    updatedAt: date(config, 'updatedAt') as string,
    name: text(config, 'name', 128, true),
    ext: ext as SystemConfigFormat,
    remark: text(config, 'remark', 255),
    builtIn: bool(config, 'isBuiltIn'),
  };
}

export function parseSystemConfigDetail(value: unknown): SystemConfigDetail {
  const config = record(value, 'system configuration');
  return {
    ...parseSystemConfigSummary(config),
    content: untrimmedText(config, 'content', MAX_SYSTEM_CONFIG_BYTES),
  };
}

export function serializeSystemConfigWrite(
  values: SystemConfigWriteValues,
): Record<string, unknown> {
  const name = values.name.trim();
  const remark = values.remark?.trim() ?? '';
  if (!name || [...name].length > 128 || /[/\\\0]/.test(name) || [...remark].length > 255) {
    throw new OperationsContractError('system configuration values are invalid');
  }
  if (new TextEncoder().encode(values.content).length > MAX_SYSTEM_CONFIG_BYTES) {
    throw new OperationsContractError('system configuration content is too large');
  }
  return { name, ext: values.ext, content: values.content, remark };
}

export function systemConfigFormValues(config: SystemConfigDetail): SystemConfigWriteValues {
  return {
    name: config.name,
    ext: config.ext,
    content: config.content,
    remark: config.remark,
  };
}
