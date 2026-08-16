import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { queryKeys } from '@/shared/query/client';
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

export type AdministrationResource = 'departments' | 'menus' | 'posts' | 'roles' | 'users';

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

export function useMenuTree(enabled = true) {
  return useQuery({
    queryKey: queryKeys.administrationTree('menus'),
    queryFn: administrationAPI.menus.tree,
    enabled,
    staleTime: 30_000,
  });
}
