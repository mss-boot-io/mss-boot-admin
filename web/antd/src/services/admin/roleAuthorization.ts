import { getRoleAuthorizeRoleId, postRoleAuthorizeRoleId } from './role';

type RoleAuthorizationClient = {
  get: (roleID: string, options?: Record<string, unknown>) => Promise<unknown>;
  post: (roleID: string, paths: string[], options?: Record<string, unknown>) => Promise<unknown>;
};

export type RoleAuthorizationResource = {
  roleID: string;
  paths: string[];
  revision: string;
  versioned: boolean;
};

export type RoleAuthorizationAdapter = {
  load: (roleID: string) => Promise<RoleAuthorizationResource>;
  save: (
    paths: readonly string[],
    base: RoleAuthorizationResource,
  ) => Promise<RoleAuthorizationResource>;
};

export class RoleAuthorizationRevisionConflictError extends Error {
  current?: RoleAuthorizationResource;

  constructor(current?: RoleAuthorizationResource) {
    super('Role authorization changed in another session');
    this.name = 'RoleAuthorizationRevisionConflictError';
    this.current = current;
  }
}

const isRecord = (value: unknown): value is Record<string, any> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

export const normalizeRoleAuthorizationPaths = (value: unknown): string[] => {
  if (!Array.isArray(value)) return [];
  return Array.from(
    new Set(
      value
        .filter((path): path is string => typeof path === 'string')
        .map((path) => path.trim())
        .filter(Boolean),
    ),
  ).sort();
};

const normalizeRevision = (value: unknown) => {
  if (typeof value === 'string' && /^(0|[1-9]\d*)$/.test(value.trim())) return value.trim();
  if (typeof value === 'number' && Number.isSafeInteger(value) && value >= 0) return String(value);
  return undefined;
};

export const parseRoleAuthorizationResource = (
  value: unknown,
  expectedRoleID: string,
): RoleAuthorizationResource => {
  const outer = isRecord(value) ? value : {};
  const record =
    !('roleID' in outer) && !('paths' in outer) && !('revision' in outer) && isRecord(outer.data)
      ? outer.data
      : outer;
  const meta = isRecord(record._meta) ? record._meta : {};
  const explicitRoleID = record.roleID ?? meta.roleID;
  const roleIDValue = explicitRoleID ?? expectedRoleID;
  const roleID = typeof roleIDValue === 'string' ? roleIDValue.trim() : '';
  if (!roleID || roleID !== expectedRoleID) {
    throw new Error('Invalid role authorization resource identity');
  }
  const revision = normalizeRevision(record.revision ?? meta.revision);
  const declaresRevision = record.revision !== undefined || meta.revision !== undefined;
  if (declaresRevision && revision === undefined) {
    throw new Error('Invalid role authorization resource revision');
  }
  if (revision !== undefined && (explicitRoleID === undefined || !Array.isArray(record.paths))) {
    throw new Error('Invalid canonical role authorization resource');
  }

  return {
    roleID,
    paths: normalizeRoleAuthorizationPaths(record.paths),
    revision: revision || '',
    versioned: revision !== undefined,
  };
};

export const formatRoleAuthorizationETag = (resource: RoleAuthorizationResource) =>
  JSON.stringify(`role-authorization-${resource.roleID}-${resource.revision}`);

const requestOptions = { skipErrorHandler: true };

const mutationOptions = (base: RoleAuthorizationResource) => ({
  skipErrorHandler: true,
  headers: {
    'Content-Type': 'application/json',
    ...(base.versioned ? { 'If-Match': formatRoleAuthorizationETag(base) } : {}),
  },
});

const defaultClient: RoleAuthorizationClient = {
  get: (roleID, options) => getRoleAuthorizeRoleId({ roleID }, options),
  post: (roleID, paths, options) => postRoleAuthorizeRoleId({ roleID }, { paths }, options),
};

const getStatus = (error: any) =>
  error?.response?.status ?? error?.status ?? error?.code ?? error?.info?.errorCode;

const isRevisionConflictResponse = (error: any) => {
  const code = error?.info?.errorCode ?? error?.response?.data?.errorCode;
  return (
    getStatus(error) === 412 ||
    String(getStatus(error)) === '412' ||
    code === 'AUTHORIZATION_REVISION_CONFLICT'
  );
};

const getConflictCurrent = (error: any, roleID: string): RoleAuthorizationResource | undefined => {
  const current =
    error?.info?.data?.current ??
    error?.response?.data?.data?.current ??
    error?.response?.data?.current;
  if (!current) return undefined;
  try {
    return parseRoleAuthorizationResource(current, roleID);
  } catch {
    return undefined;
  }
};

export const isRoleAuthorizationRevisionConflictError = (
  error: unknown,
): error is RoleAuthorizationRevisionConflictError =>
  error instanceof RoleAuthorizationRevisionConflictError;

export const createRoleAuthorizationAdapter = (
  client: RoleAuthorizationClient = defaultClient,
): RoleAuthorizationAdapter => {
  const load = async (roleID: string) =>
    parseRoleAuthorizationResource(await client.get(roleID, requestOptions), roleID);

  return {
    load,
    save: async (paths, base) => {
      const normalizedPaths = normalizeRoleAuthorizationPaths(paths);
      try {
        const next = parseRoleAuthorizationResource(
          await client.post(base.roleID, normalizedPaths, mutationOptions(base)),
          base.roleID,
        );
        // Pre-revision servers return an empty or legacy success body. Re-read
        // instead of predicting canonical authorization during rolling deploys.
        return next.versioned ? next : load(base.roleID);
      } catch (error) {
        if (isRevisionConflictResponse(error)) {
          throw new RoleAuthorizationRevisionConflictError(getConflictCurrent(error, base.roleID));
        }
        throw error;
      }
    },
  };
};
