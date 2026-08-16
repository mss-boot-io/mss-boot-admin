export const ADMIN_PAGE_SIZES = [20, 50, 100] as const;
export const MAX_ADMIN_PAGE_SIZE = 100;
export const MAX_ADMIN_TREE_DEPTH = 8;
export const MAX_ADMIN_TREE_NODES = 2_000;

export type AdminPageSize = (typeof ADMIN_PAGE_SIZES)[number];
export type AdministrationStatus = '' | 'disabled' | 'enabled' | 'locked';
export type AdministrationStatusFilter = AdministrationStatus | 'all';

export interface AdministrationListParams {
  current: number;
  pageSize: AdminPageSize;
  name?: string;
  status?: AdministrationStatusFilter;
}

export interface AdministrationPage<T> {
  data: T[];
  total: number;
  current: number;
  pageSize: number;
}

export interface AdministrationNode {
  id: string;
  name: string;
  parentID: string;
  status: AdministrationStatus;
  sort: number;
  children?: AdministrationNode[];
}

export interface AdministrationSelectOption {
  disabled: boolean;
  label: string;
  value: string;
}

/**
 * Resolve a relation through its embedded projection or a loaded option map.
 * Identifiers remain transport values and are deliberately never UI fallbacks.
 */
export function administrationReferenceName(
  reference: { name?: string } | undefined,
  id: string | undefined,
  namesByID: ReadonlyMap<string, string>,
): string {
  const embeddedName = reference?.name?.trim();
  if (embeddedName) return embeddedName;
  const loadedName = id ? namesByID.get(id)?.trim() : undefined;
  return loadedName || '—';
}

interface AdministrationSelectOptions<T> {
  disabled?: ReadonlySet<string>;
  include?: (item: T) => boolean;
  label?: (item: T) => string;
}

export interface RoleSummary {
  id: string;
  name: string;
  remark: string;
  root: boolean;
  default: boolean;
  status: AdministrationStatus;
  updatedAt?: string;
}

export interface RoleWriteValues {
  name: string;
  remark?: string;
  status: AdministrationStatus;
}

export interface UserSummary {
  id: string;
  username: string;
  name: string;
  email: string;
  avatar?: string;
  roleID: string;
  role?: Pick<RoleSummary, 'default' | 'id' | 'name' | 'root' | 'status'>;
  departmentID: string;
  department?: { id: string; name: string };
  postID: string;
  post?: { id: string; name: string };
  status: AdministrationStatus;
  updatedAt?: string;
}

export interface UserWriteValues {
  username: string;
  name?: string;
  email?: string;
  password?: string;
  roleID?: string;
  departmentID?: string;
  postID?: string;
  status: AdministrationStatus;
}

export interface MenuSummary extends AdministrationNode {
  path: string;
  permission: string;
  method: string;
  type: 'API' | 'COMPONENT' | 'DIRECTORY' | 'MENU';
  component: string;
  icon: string;
  hideInMenu: boolean;
  children?: MenuSummary[];
}

export interface MenuWriteValues {
  parentID?: string;
  name: string;
  path: string;
  permission?: string;
  method?: string;
  type: MenuSummary['type'];
  component?: string;
  icon?: string;
  hideInMenu?: boolean;
  status: AdministrationStatus;
  sort?: number;
}

export interface AdministrationAPIReference {
  id: string;
  name: string;
  path: string;
  method: string;
}

export interface DepartmentSummary extends AdministrationNode {
  code: string;
  leaderID: string;
  phone: string;
  email: string;
  children?: DepartmentSummary[];
}

export interface DepartmentWriteValues {
  parentID?: string;
  name: string;
  code: string;
  leaderID?: string;
  phone?: string;
  email?: string;
  status: AdministrationStatus;
  sort?: number;
}

export type DataScope =
  | 'all'
  | 'currentAndChildrenDept'
  | 'currentDept'
  | 'customDept'
  | 'self'
  | 'selfAndAllChildren'
  | 'selfAndChildren';

export interface PostSummary extends AdministrationNode {
  code: string;
  dataScope: DataScope;
  deptIDS: string[];
  children?: PostSummary[];
}

export interface PostWriteValues {
  parentID?: string;
  name: string;
  code: string;
  dataScope: DataScope;
  deptIDS?: string[];
  status: AdministrationStatus;
  sort?: number;
}

export interface RoleAuthorizationResource {
  roleID: string;
  paths: string[];
  revision: string;
  etag?: string;
}

export class AdministrationContractError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AdministrationContractError';
  }
}

function record(value: unknown, label: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new AdministrationContractError(`${label} must be an object`);
  }
  return value as Record<string, unknown>;
}

function text(value: Record<string, unknown>, key: string, max: number, required = false): string {
  const candidate = value[key];
  if (candidate === undefined || candidate === null) {
    if (required) throw new AdministrationContractError(`${key} is required`);
    return '';
  }
  if (typeof candidate !== 'string') {
    throw new AdministrationContractError(`${key} must be a string`);
  }
  const normalized = candidate.trim();
  if ((required && !normalized) || [...normalized].length > max) {
    throw new AdministrationContractError(`${key} is invalid`);
  }
  return normalized;
}

function identifier(value: Record<string, unknown>, key = 'id', required = true): string {
  return text(value, key, 64, required);
}

function bool(value: Record<string, unknown>, key: string): boolean {
  const candidate = value[key];
  if (candidate === undefined || candidate === null) return false;
  if (typeof candidate !== 'boolean') {
    throw new AdministrationContractError(`${key} must be a boolean`);
  }
  return candidate;
}

function integer(value: Record<string, unknown>, key: string, fallback = 0): number {
  const candidate = value[key];
  if (candidate === undefined || candidate === null) return fallback;
  if (
    typeof candidate !== 'number' ||
    !Number.isSafeInteger(candidate) ||
    Math.abs(candidate) > 1_000_000
  ) {
    throw new AdministrationContractError(`${key} must be a bounded integer`);
  }
  return candidate as number;
}

function status(value: Record<string, unknown>): AdministrationStatus {
  const candidate = value.status ?? '';
  if (!['', 'disabled', 'enabled', 'locked'].includes(candidate as string)) {
    throw new AdministrationContractError('status is invalid');
  }
  return candidate as AdministrationStatus;
}

function optionalDate(value: Record<string, unknown>, key: string): string | undefined {
  const candidate = text(value, key, 64);
  if (!candidate) return undefined;
  if (!Number.isFinite(Date.parse(candidate))) {
    throw new AdministrationContractError(`${key} is not a date`);
  }
  return candidate;
}

function namedReference(value: unknown, label: string): { id: string; name: string } | undefined {
  if (value === undefined || value === null) return undefined;
  const candidate = record(value, label);
  return {
    id: identifier(candidate),
    name: text(candidate, 'name', 255, true),
  };
}

export function isAdminPageSize(value: number): value is AdminPageSize {
  return ADMIN_PAGE_SIZES.includes(value as AdminPageSize);
}

export function parseAdministrationPage<T>(
  value: unknown,
  params: Pick<AdministrationListParams, 'current' | 'pageSize'>,
  parseItem: (item: unknown) => T,
): AdministrationPage<T> {
  const page = record(value, 'page');
  if (!Array.isArray(page.data) || page.data.length > MAX_ADMIN_PAGE_SIZE) {
    throw new AdministrationContractError('page data is invalid');
  }
  const total = integer(page, 'total');
  const current = integer(page, 'current', params.current) || params.current;
  const pageSize = integer(page, 'pageSize', params.pageSize) || params.pageSize;
  if (total < 0 || current < 1 || pageSize < 1 || pageSize > MAX_ADMIN_PAGE_SIZE) {
    throw new AdministrationContractError('page metadata is invalid');
  }
  return { data: page.data.map(parseItem), total, current, pageSize };
}

export function parseRole(value: unknown): RoleSummary {
  const role = record(value, 'role');
  return {
    id: identifier(role),
    name: text(role, 'name', 255, true),
    remark: text(role, 'remark', 4_096),
    root: bool(role, 'root'),
    default: bool(role, 'default'),
    status: status(role),
    updatedAt: optionalDate(role, 'updatedAt'),
  };
}

export function parseUser(value: unknown): UserSummary {
  const user = record(value, 'user');
  const rawRole = user.role === undefined || user.role === null ? undefined : parseRole(user.role);
  return {
    id: identifier(user),
    username: text(user, 'username', 20, true),
    name: text(user, 'name', 100),
    email: text(user, 'email', 100),
    avatar: text(user, 'avatar', 255) || undefined,
    roleID: identifier(user, 'roleID', false),
    role: rawRole
      ? {
          id: rawRole.id,
          name: rawRole.name,
          root: rawRole.root,
          default: rawRole.default,
          status: rawRole.status,
        }
      : undefined,
    departmentID: identifier(user, 'departmentID', false),
    department: namedReference(user.department, 'department'),
    postID: identifier(user, 'postID', false),
    post: namedReference(user.post, 'post'),
    status: status(user),
    updatedAt: optionalDate(user, 'updatedAt'),
  };
}

interface TreeParseState {
  count: number;
}

function parseTree<T extends AdministrationNode>(
  value: unknown,
  depth: number,
  state: TreeParseState,
  ancestry: ReadonlySet<string>,
  label: string,
  parseFields: (candidate: Record<string, unknown>) => Omit<T, keyof AdministrationNode>,
): T {
  if (depth > MAX_ADMIN_TREE_DEPTH || state.count >= MAX_ADMIN_TREE_NODES) {
    throw new AdministrationContractError(`${label} tree exceeds its limit`);
  }
  state.count += 1;
  const candidate = record(value, label);
  const id = identifier(candidate);
  if (ancestry.has(id)) throw new AdministrationContractError(`${label} tree contains a cycle`);
  const nextAncestry = new Set(ancestry);
  nextAncestry.add(id);
  const rawChildren = candidate.children;
  if (rawChildren !== undefined && !Array.isArray(rawChildren)) {
    throw new AdministrationContractError(`${label} children must be an array`);
  }
  const base: AdministrationNode = {
    id,
    name: text(candidate, 'name', 255, true),
    parentID: identifier(candidate, 'parentID', false),
    status: status(candidate),
    sort: integer(candidate, 'sort'),
  };
  const children = (rawChildren ?? []).map((child) =>
    parseTree<T>(child, depth + 1, state, nextAncestry, label, parseFields),
  );
  return {
    ...base,
    ...parseFields(candidate),
    ...(children.length ? { children } : {}),
  } as T;
}

export function parseMenu(value: unknown): MenuSummary {
  return parseTree<MenuSummary>(value, 0, { count: 0 }, new Set(), 'menu', (menu) => {
    const type = text(menu, 'type', 20, true);
    if (!['API', 'COMPONENT', 'DIRECTORY', 'MENU'].includes(type)) {
      throw new AdministrationContractError('menu type is invalid');
    }
    return {
      path: text(menu, 'path', 255),
      permission: text(menu, 'permission', 255),
      method: text(menu, 'method', 10) || 'GET',
      type: type as MenuSummary['type'],
      component: text(menu, 'component', 255),
      icon: text(menu, 'icon', 255),
      hideInMenu: bool(menu, 'hideInMenu'),
    };
  });
}

export function parseAdministrationAPIReference(value: unknown): AdministrationAPIReference {
  const api = record(value, 'api reference');
  const method = text(api, 'method', 10, true).toUpperCase();
  if (!['DELETE', 'GET', 'HEAD', 'OPTIONS', 'PATCH', 'POST', 'PUT'].includes(method)) {
    throw new AdministrationContractError('api reference method is invalid');
  }
  return {
    id: identifier(api),
    name: text(api, 'name', 255) || text(api, 'path', 255, true),
    path: text(api, 'path', 255, true),
    method,
  };
}

export function parseAdministrationAPIPage(
  value: unknown,
  current: number,
  pageSize: AdminPageSize,
): AdministrationPage<AdministrationAPIReference> {
  return parseAdministrationPage(value, { current, pageSize }, parseAdministrationAPIReference);
}

export function administrationAPIReferenceKey(
  reference: Pick<AdministrationAPIReference, 'method' | 'path'>,
): string {
  return `${reference.method.toUpperCase()}---${reference.path}`;
}

export function parseDepartment(value: unknown): DepartmentSummary {
  return parseTree<DepartmentSummary>(
    value,
    0,
    { count: 0 },
    new Set(),
    'department',
    (department) => ({
      code: text(department, 'code', 255, true),
      leaderID: identifier(department, 'leaderID', false),
      phone: text(department, 'phone', 50),
      email: text(department, 'email', 255),
    }),
  );
}

const DATA_SCOPES = [
  'all',
  'currentAndChildrenDept',
  'currentDept',
  'customDept',
  'self',
  'selfAndAllChildren',
  'selfAndChildren',
] as const;

export function parsePost(value: unknown): PostSummary {
  return parseTree<PostSummary>(value, 0, { count: 0 }, new Set(), 'post', (post) => {
    const scope = text(post, 'dataScope', 50) || 'self';
    if (!DATA_SCOPES.includes(scope as DataScope)) {
      throw new AdministrationContractError('post data scope is invalid');
    }
    if (post.deptIDS !== undefined && !Array.isArray(post.deptIDS)) {
      throw new AdministrationContractError('post department scope is invalid');
    }
    const deptIDS = (post.deptIDS ?? []).map((entry) => {
      if (typeof entry !== 'string' || !entry.trim() || entry.length > 64) {
        throw new AdministrationContractError('post department scope contains an invalid ID');
      }
      return entry.trim();
    });
    if (deptIDS.length > MAX_ADMIN_TREE_NODES || new Set(deptIDS).size !== deptIDS.length) {
      throw new AdministrationContractError('post department scope is invalid');
    }
    return {
      code: text(post, 'code', 255, true),
      dataScope: scope as DataScope,
      deptIDS,
    };
  });
}

export function flattenAdministrationTree<T extends AdministrationNode>(items: readonly T[]): T[] {
  return items.flatMap((item) => [
    item,
    ...flattenAdministrationTree((item.children ?? []) as T[]),
  ]);
}

export function administrationSubtreeIDs<T extends AdministrationNode>(
  items: readonly T[],
  rootID: string,
): Set<string> {
  for (const item of items) {
    if (item.id === rootID) {
      return new Set(flattenAdministrationTree([item]).map((entry) => entry.id));
    }
    const descendants = administrationSubtreeIDs((item.children ?? []) as T[], rootID);
    if (descendants.size > 0) return descendants;
  }
  return new Set();
}

export function administrationSelectOptions<T extends AdministrationNode>(
  items: readonly T[],
  options: AdministrationSelectOptions<T> = {},
  ancestors: readonly string[] = [],
): AdministrationSelectOption[] {
  return items.flatMap((item) => {
    if (options.include && !options.include(item)) return [];
    const path = [...ancestors, options.label?.(item) ?? item.name];
    return [
      {
        value: item.id,
        label: path.join(' / '),
        disabled: item.status !== 'enabled' || Boolean(options.disabled?.has(item.id)),
      },
      ...administrationSelectOptions((item.children ?? []) as T[], options, path),
    ];
  });
}

function cleanRequired(value: string, field: string, max: number): string {
  const normalized = value.trim();
  if (!normalized || [...normalized].length > max) {
    throw new AdministrationContractError(`${field} is invalid`);
  }
  return normalized;
}

function cleanOptional(value: string | undefined, max: number): string {
  const normalized = value?.trim() ?? '';
  if ([...normalized].length > max) throw new AdministrationContractError('field is too long');
  return normalized;
}

function writeStatus(value: AdministrationStatus): AdministrationStatus {
  if (!['disabled', 'enabled', 'locked'].includes(value)) {
    throw new AdministrationContractError('status is invalid');
  }
  return value;
}

export function serializeRoleWrite(values: RoleWriteValues) {
  return {
    name: cleanRequired(values.name, 'name', 255),
    remark: cleanOptional(values.remark, 4_096),
    status: writeStatus(values.status),
  };
}

export function serializeUserWrite(values: UserWriteValues, mode: 'create' | 'edit') {
  const username = cleanRequired(values.username, 'username', 20);
  if (!/^[A-Za-z0-9_]{3,20}$/.test(username)) {
    throw new AdministrationContractError('username is invalid');
  }
  const email = cleanOptional(values.email, 100);
  if (email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    throw new AdministrationContractError('email is invalid');
  }
  const password = cleanOptional(values.password, 128);
  if (
    mode === 'create' &&
    (password.length < 8 || !/[A-Za-z]/.test(password) || !/\d/.test(password))
  ) {
    throw new AdministrationContractError('password is invalid');
  }
  return {
    username,
    name: cleanOptional(values.name, 100),
    email,
    ...(mode === 'create' ? { password } : {}),
    roleID: cleanOptional(values.roleID, 64),
    departmentID: cleanOptional(values.departmentID, 64),
    postID: cleanOptional(values.postID, 64),
    status: writeStatus(values.status),
  };
}

export function serializeMenuWrite(values: MenuWriteValues) {
  const method = cleanOptional(values.method, 10).toUpperCase() || 'GET';
  if (!['DELETE', 'GET', 'PATCH', 'POST', 'PUT'].includes(method)) {
    throw new AdministrationContractError('menu method is invalid');
  }
  return {
    parentID: cleanOptional(values.parentID, 64),
    name: cleanRequired(values.name, 'name', 255),
    path: cleanRequired(values.path, 'path', 255),
    permission: cleanOptional(values.permission, 255),
    method,
    type: values.type,
    component: cleanOptional(values.component, 255),
    icon: cleanOptional(values.icon, 255),
    hideInMenu: Boolean(values.hideInMenu),
    status: writeStatus(values.status),
    sort: Number.isSafeInteger(values.sort) ? values.sort : 0,
  };
}

export function serializeDepartmentWrite(values: DepartmentWriteValues) {
  return {
    parentID: cleanOptional(values.parentID, 64),
    name: cleanRequired(values.name, 'name', 255),
    code: cleanRequired(values.code, 'code', 255),
    leaderID: cleanOptional(values.leaderID, 64),
    phone: cleanOptional(values.phone, 50),
    email: cleanOptional(values.email, 255),
    status: writeStatus(values.status),
    sort: Number.isSafeInteger(values.sort) ? values.sort : 0,
  };
}

export function serializePostWrite(values: PostWriteValues) {
  if (!DATA_SCOPES.includes(values.dataScope)) {
    throw new AdministrationContractError('post data scope is invalid');
  }
  const deptIDS = [...new Set(values.deptIDS ?? [])].map((id) => cleanRequired(id, 'deptID', 64));
  if (deptIDS.length > MAX_ADMIN_TREE_NODES) {
    throw new AdministrationContractError('post department scope is too large');
  }
  return {
    parentID: cleanOptional(values.parentID, 64),
    name: cleanRequired(values.name, 'name', 255),
    code: cleanRequired(values.code, 'code', 255),
    dataScope: values.dataScope,
    deptIDS,
    status: writeStatus(values.status),
    sort: Number.isSafeInteger(values.sort) ? values.sort : 0,
  };
}

export function parseRoleAuthorization(value: unknown, etag?: string): RoleAuthorizationResource {
  const resource = record(value, 'role authorization');
  if (!Array.isArray(resource.paths) || resource.paths.length > MAX_ADMIN_TREE_NODES) {
    throw new AdministrationContractError('authorization paths are invalid');
  }
  const paths = resource.paths.map((path) => {
    if (typeof path !== 'string' || !path.trim() || path.length > 255) {
      throw new AdministrationContractError('authorization path is invalid');
    }
    return path.trim();
  });
  if (new Set(paths).size !== paths.length) {
    throw new AdministrationContractError('authorization paths are duplicated');
  }
  return {
    roleID: identifier(resource, 'roleID'),
    revision: text(resource, 'revision', 128, true),
    paths: paths.sort(),
    etag,
  };
}
