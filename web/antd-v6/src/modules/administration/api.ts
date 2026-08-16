import { request } from '@umijs/max';
import {
  type AdministrationAPIReference,
  type AdministrationListParams,
  type AdministrationPage,
  administrationAPIReferenceKey,
  type DepartmentSummary,
  type DepartmentWriteValues,
  type MenuSummary,
  type MenuWriteValues,
  type PostSummary,
  type PostWriteValues,
  parseAdministrationAPIPage,
  parseAdministrationAPIReference,
  parseAdministrationPage,
  parseDepartment,
  parseMenu,
  parsePost,
  parseRole,
  parseRoleAuthorization,
  parseUser,
  type RoleAuthorizationResource,
  type RoleSummary,
  type RoleWriteValues,
  serializeDepartmentWrite,
  serializeMenuWrite,
  serializePostWrite,
  serializeRoleWrite,
  serializeUserWrite,
  type UserSummary,
  type UserWriteValues,
} from './contract';

interface AdministrationRequestOptions {
  method: 'DELETE' | 'GET' | 'POST' | 'PUT';
  data?: unknown;
  headers?: Record<string, string>;
  params?: Record<string, boolean | number | string | string[] | undefined>;
  skipErrorHandler: true;
}

export type AdministrationRequestClient = (
  path: string,
  options: AdministrationRequestOptions,
) => Promise<unknown>;

function pageParams(params: AdministrationListParams) {
  return {
    current: params.current,
    pageSize: params.pageSize,
    name: params.name?.trim() || undefined,
    status: !params.status || params.status === 'all' ? undefined : params.status,
  };
}

function entityPath(resource: string, id: string): string {
  return `/${resource}/${encodeURIComponent(id)}`;
}

export class RoleAuthorizationRevisionConflictError extends Error {
  current?: RoleAuthorizationResource;

  constructor(current?: RoleAuthorizationResource) {
    super('Role authorization changed in another session');
    this.name = 'RoleAuthorizationRevisionConflictError';
    this.current = current;
  }
}

function authorizationETag(resource: RoleAuthorizationResource): string {
  return JSON.stringify(`role-authorization-${resource.roleID}-${resource.revision}`);
}

function requestStatus(error: unknown): number | undefined {
  return (error as { response?: { status?: number } })?.response?.status;
}

function authorizationConflict(error: unknown): RoleAuthorizationResource | undefined {
  const current = (error as { response?: { data?: { data?: { current?: unknown } } } })?.response
    ?.data?.data?.current;
  if (!current) return undefined;
  try {
    return parseRoleAuthorization(current);
  } catch {
    return undefined;
  }
}

export function createAdministrationAPI(client: AdministrationRequestClient) {
  const roles = {
    list: async (params: AdministrationListParams): Promise<AdministrationPage<RoleSummary>> =>
      parseAdministrationPage(
        await client('/roles', {
          method: 'GET',
          params: pageParams(params),
          skipErrorHandler: true,
        }),
        params,
        parseRole,
      ),
    get: async (id: string): Promise<RoleSummary> =>
      parseRole(await client(entityPath('roles', id), { method: 'GET', skipErrorHandler: true })),
    create: async (values: RoleWriteValues): Promise<RoleSummary> =>
      parseRole(
        await client('/roles', {
          method: 'POST',
          data: serializeRoleWrite(values),
          skipErrorHandler: true,
        }),
      ),
    update: async (id: string, values: RoleWriteValues): Promise<RoleSummary> =>
      parseRole(
        await client(entityPath('roles', id), {
          method: 'PUT',
          data: serializeRoleWrite(values),
          skipErrorHandler: true,
        }),
      ),
    remove: async (id: string): Promise<void> => {
      await client(entityPath('roles', id), { method: 'DELETE', skipErrorHandler: true });
    },
    loadAuthorization: async (roleID: string): Promise<RoleAuthorizationResource> =>
      parseRoleAuthorization(
        await client(`/role/authorize/${encodeURIComponent(roleID)}`, {
          method: 'GET',
          skipErrorHandler: true,
        }),
      ),
    saveAuthorization: async (
      base: RoleAuthorizationResource,
      paths: readonly string[],
    ): Promise<RoleAuthorizationResource> => {
      try {
        return parseRoleAuthorization(
          await client(`/role/authorize/${encodeURIComponent(base.roleID)}`, {
            method: 'POST',
            data: { paths: [...new Set(paths.map((path) => path.trim()).filter(Boolean))].sort() },
            headers: { 'If-Match': authorizationETag(base) },
            skipErrorHandler: true,
          }),
        );
      } catch (error) {
        if (requestStatus(error) === 412) {
          throw new RoleAuthorizationRevisionConflictError(authorizationConflict(error));
        }
        throw error;
      }
    },
  };

  const users = {
    list: async (params: AdministrationListParams): Promise<AdministrationPage<UserSummary>> =>
      parseAdministrationPage(
        await client('/users', {
          method: 'GET',
          params: pageParams(params),
          skipErrorHandler: true,
        }),
        params,
        parseUser,
      ),
    get: async (id: string): Promise<UserSummary> =>
      parseUser(await client(entityPath('users', id), { method: 'GET', skipErrorHandler: true })),
    create: async (values: UserWriteValues): Promise<UserSummary> =>
      parseUser(
        await client('/users', {
          method: 'POST',
          data: serializeUserWrite(values, 'create'),
          skipErrorHandler: true,
        }),
      ),
    update: async (id: string, values: UserWriteValues): Promise<UserSummary> =>
      parseUser(
        await client(entityPath('users', id), {
          method: 'PUT',
          data: serializeUserWrite(values, 'edit'),
          skipErrorHandler: true,
        }),
      ),
    remove: async (id: string): Promise<void> => {
      await client(entityPath('users', id), { method: 'DELETE', skipErrorHandler: true });
    },
    resetPassword: async (id: string, password: string): Promise<void> => {
      await client(`/user/${encodeURIComponent(id)}/password-reset`, {
        method: 'PUT',
        data: { password },
        skipErrorHandler: true,
      });
    },
  };

  const menus = {
    list: async (params: AdministrationListParams): Promise<AdministrationPage<MenuSummary>> =>
      parseAdministrationPage(
        await client('/menus', {
          method: 'GET',
          params: { ...pageParams(params), type: ['DIRECTORY', 'MENU', 'COMPONENT'] },
          skipErrorHandler: true,
        }),
        params,
        parseMenu,
      ),
    tree: async (): Promise<MenuSummary[]> => {
      const value = await client('/menu/tree', { method: 'GET', skipErrorHandler: true });
      if (!Array.isArray(value) || value.length > 2_000) throw new Error('Menu tree is invalid');
      return value.map(parseMenu);
    },
    get: async (id: string): Promise<MenuSummary> =>
      parseMenu(await client(entityPath('menus', id), { method: 'GET', skipErrorHandler: true })),
    create: async (values: MenuWriteValues): Promise<MenuSummary> =>
      parseMenu(
        await client('/menus', {
          method: 'POST',
          data: serializeMenuWrite(values),
          skipErrorHandler: true,
        }),
      ),
    update: async (id: string, values: MenuWriteValues): Promise<MenuSummary> =>
      parseMenu(
        await client(entityPath('menus', id), {
          method: 'PUT',
          data: serializeMenuWrite(values),
          skipErrorHandler: true,
        }),
      ),
    remove: async (id: string): Promise<void> => {
      await client(entityPath('menus', id), { method: 'DELETE', skipErrorHandler: true });
    },
    listAPIReferences: async (): Promise<AdministrationAPIReference[]> => {
      const references: AdministrationAPIReference[] = [];
      let current = 1;
      let total = 0;
      do {
        const page = parseAdministrationAPIPage(
          await client('/apis', {
            method: 'GET',
            params: { current, pageSize: 100 },
            skipErrorHandler: true,
          }),
          current,
          100,
        );
        if (current === 1) {
          total = page.total;
          if (total > 2_000) throw new Error('API reference catalog exceeds its limit');
        }
        if (page.data.length === 0 && references.length < total) {
          throw new Error('API reference catalog pagination is incomplete');
        }
        references.push(...page.data);
        current += 1;
      } while (references.length < total);

      const keys = new Set(references.map(administrationAPIReferenceKey));
      if (keys.size !== references.length) {
        throw new Error('API reference catalog contains duplicate method and path pairs');
      }
      return references;
    },
    loadBoundAPIs: async (id: string): Promise<AdministrationAPIReference[]> => {
      const value = await client(`/menu/api/${encodeURIComponent(id)}`, {
        method: 'GET',
        skipErrorHandler: true,
      });
      if (!Array.isArray(value) || value.length > 2_000) {
        throw new Error('Bound API reference list is invalid');
      }
      return value.map(parseAdministrationAPIReference);
    },
    bindAPIs: async (id: string, paths: readonly string[]): Promise<void> => {
      await client('/menu/bind-api', {
        method: 'POST',
        data: {
          menuID: id,
          paths: [...new Set(paths.map((path) => path.trim()).filter(Boolean))].sort(),
        },
        skipErrorHandler: true,
      });
    },
  };

  const departments = {
    list: async (
      params: AdministrationListParams,
    ): Promise<AdministrationPage<DepartmentSummary>> =>
      parseAdministrationPage(
        await client('/departments', {
          method: 'GET',
          params: {
            name: params.name?.trim() || undefined,
            status: !params.status || params.status === 'all' ? undefined : params.status,
            page: params.current,
            pageSize: params.pageSize,
            parentID: '',
          },
          skipErrorHandler: true,
        }),
        params,
        parseDepartment,
      ),
    get: async (id: string): Promise<DepartmentSummary> =>
      parseDepartment(
        await client(entityPath('departments', id), { method: 'GET', skipErrorHandler: true }),
      ),
    create: async (values: DepartmentWriteValues): Promise<DepartmentSummary> =>
      parseDepartment(
        await client('/departments', {
          method: 'POST',
          data: serializeDepartmentWrite(values),
          skipErrorHandler: true,
        }),
      ),
    update: async (id: string, values: DepartmentWriteValues): Promise<DepartmentSummary> =>
      parseDepartment(
        await client(entityPath('departments', id), {
          method: 'PUT',
          data: serializeDepartmentWrite(values),
          skipErrorHandler: true,
        }),
      ),
    remove: async (id: string): Promise<void> => {
      await client(entityPath('departments', id), {
        method: 'DELETE',
        skipErrorHandler: true,
      });
    },
  };

  const posts = {
    list: async (params: AdministrationListParams): Promise<AdministrationPage<PostSummary>> =>
      parseAdministrationPage(
        await client('/posts', {
          method: 'GET',
          params: {
            name: params.name?.trim() || undefined,
            status: !params.status || params.status === 'all' ? undefined : params.status,
            page: params.current,
            pageSize: params.pageSize,
            parentID: '',
          },
          skipErrorHandler: true,
        }),
        params,
        parsePost,
      ),
    get: async (id: string): Promise<PostSummary> =>
      parsePost(await client(entityPath('posts', id), { method: 'GET', skipErrorHandler: true })),
    create: async (values: PostWriteValues): Promise<PostSummary> =>
      parsePost(
        await client('/posts', {
          method: 'POST',
          data: serializePostWrite(values),
          skipErrorHandler: true,
        }),
      ),
    update: async (id: string, values: PostWriteValues): Promise<PostSummary> =>
      parsePost(
        await client(entityPath('posts', id), {
          method: 'PUT',
          data: serializePostWrite(values),
          skipErrorHandler: true,
        }),
      ),
    remove: async (id: string): Promise<void> => {
      await client(entityPath('posts', id), { method: 'DELETE', skipErrorHandler: true });
    },
  };

  return { departments, menus, posts, roles, users };
}

export const administrationAPI = createAdministrationAPI((path, options) =>
  request<unknown>(path, options),
);
