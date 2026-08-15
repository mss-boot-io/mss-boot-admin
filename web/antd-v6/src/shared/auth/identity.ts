import type { CurrentUser, CurrentUserRole } from './types';

export class IdentityContractError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'IdentityContractError';
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function optionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value : undefined;
}

function normalizeRole(value: unknown): CurrentUserRole | undefined {
  if (!isRecord(value)) return undefined;
  return {
    id: optionalString(value.id),
    name: optionalString(value.name),
    root: value.root === true,
    default: typeof value.default === 'boolean' ? value.default : undefined,
    status: optionalString(value.status),
  };
}

function normalizePermissions(value: unknown): Readonly<Record<string, boolean>> {
  if (!isRecord(value)) return Object.freeze({});
  const entries = Object.entries(value).flatMap(([permission, allowed]) => {
    const normalized = permission.trim();
    return normalized && typeof allowed === 'boolean' ? [[normalized, allowed] as const] : [];
  });
  return Object.freeze(Object.fromEntries(entries));
}

/**
 * Normalize the backend self-service identity projection at the transport
 * boundary. Privilege is intentionally read only from role.root and exact
 * boolean permission entries; legacy top-level root or array-shaped values do
 * not accidentally grant access.
 */
export function normalizeCurrentUser(value: unknown): CurrentUser {
  if (!isRecord(value)) throw new IdentityContractError('Current user must be an object');
  const id = optionalString(value.id);
  if (!id) throw new IdentityContractError('Current user is missing its id');

  return {
    id,
    username: optionalString(value.username),
    name: optionalString(value.name),
    avatar: optionalString(value.avatar),
    roleID: optionalString(value.roleID),
    role: normalizeRole(value.role),
    email: optionalString(value.email),
    phone: optionalString(value.phone),
    signature: optionalString(value.signature),
    title: optionalString(value.title),
    group: optionalString(value.group),
    country: optionalString(value.country),
    province: optionalString(value.province),
    city: optionalString(value.city),
    address: optionalString(value.address),
    profile: optionalString(value.profile),
    departmentID: optionalString(value.departmentID),
    postID: optionalString(value.postID),
    permissions: normalizePermissions(value.permissions),
  };
}
