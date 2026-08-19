import { queryKeys } from '@mss-admin-core/shared/query/client';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { administrationAPI } from './api';
import type {
  AdministrationListParams,
  AdministrationPage,
  DepartmentSummary,
  MenuSummary,
  PostSummary,
  RoleSummary,
  UserSummary,
} from './contract';
import { AdministrationContractError, MAX_ADMIN_TREE_NODES } from './contract';

export type AdministrationResource = 'departments' | 'menus' | 'posts' | 'roles' | 'users';
export type AdministrationCatalogResource = Exclude<AdministrationResource, 'menus'>;

interface AdministrationEntities {
  departments: DepartmentSummary;
  menus: MenuSummary;
  posts: PostSummary;
  roles: RoleSummary;
  users: UserSummary;
}

const listAPI: {
  [R in AdministrationResource]: (
    params: AdministrationListParams,
  ) => Promise<AdministrationPage<AdministrationEntities[R]>>;
} = {
  departments: administrationAPI.departments.list,
  menus: administrationAPI.menus.list,
  posts: administrationAPI.posts.list,
  roles: administrationAPI.roles.list,
  users: administrationAPI.users.list,
};

export function useAdministrationPage<R extends AdministrationResource>(
  resource: R,
  params: AdministrationListParams,
  enabled = true,
) {
  return useQuery<AdministrationPage<AdministrationEntities[R]>>({
    queryKey: queryKeys.administrationList(resource, params),
    queryFn: () => listAPI[resource](params),
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 15_000,
  });
}

export function useRoleAuthorization(roleID?: string) {
  return useQuery({
    queryKey: queryKeys.roleAuthorization(roleID ?? ''),
    queryFn: () => administrationAPI.roles.loadAuthorization(roleID as string),
    enabled: Boolean(roleID),
    staleTime: 0,
  });
}

function catalogEntries<T extends { id: string }>(items: readonly T[]): T[] {
  return items.flatMap((item) => {
    const children = (item as T & { children?: T[] }).children ?? [];
    return [item, ...catalogEntries(children)];
  });
}

export async function collectAdministrationCatalog<T extends { id: string }>(
  list: (params: AdministrationListParams) => Promise<AdministrationPage<T>>,
): Promise<T[]> {
  const result: T[] = [];
  let current = 1;
  let total = 0;

  do {
    const page = await list({ current, pageSize: 100, status: 'all' });
    if (current === 1) {
      total = page.total;
      if (total > MAX_ADMIN_TREE_NODES) {
        throw new AdministrationContractError('administration catalog exceeds its limit');
      }
    }
    if (page.data.length === 0 && result.length < total) {
      throw new AdministrationContractError('administration catalog pagination is incomplete');
    }
    result.push(...page.data);
    current += 1;
  } while (result.length < total);

  const entries = catalogEntries(result);
  if (entries.length > MAX_ADMIN_TREE_NODES) {
    throw new AdministrationContractError('administration catalog exceeds its limit');
  }
  const identifiers = new Set(entries.map((entry) => entry.id));
  if (identifiers.size !== entries.length) {
    throw new AdministrationContractError('administration catalog contains duplicate identifiers');
  }
  return result;
}

export function useAdministrationCatalog<R extends AdministrationCatalogResource>(
  resource: R,
  enabled = true,
) {
  return useQuery<AdministrationEntities[R][]>({
    queryKey: queryKeys.administrationCatalog(resource),
    queryFn: () => collectAdministrationCatalog(listAPI[resource]),
    enabled,
    staleTime: 30_000,
  });
}

export function useMenuTree(enabled = true) {
  return useQuery({
    queryKey: queryKeys.administrationTree('menus'),
    queryFn: administrationAPI.menus.tree,
    enabled,
    staleTime: 30_000,
  });
}

export function useAdministrationAPICatalog(enabled = true) {
  return useQuery({
    queryKey: ['administration', 'apis', 'catalog'],
    queryFn: administrationAPI.menus.listAPIReferences,
    enabled,
    staleTime: 30_000,
  });
}

export function useMenuAPIBindings(menuID?: string) {
  return useQuery({
    queryKey: ['administration', 'menus', menuID ?? '', 'apis'],
    queryFn: () => administrationAPI.menus.loadBoundAPIs(menuID as string),
    enabled: Boolean(menuID),
    staleTime: 0,
  });
}
