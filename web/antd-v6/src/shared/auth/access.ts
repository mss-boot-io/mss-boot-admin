import type { AuthorizedMenuItem, CurrentUser } from './types';

export interface RouteAccessMetadata {
  path?: string;
  permission?: string;
  rootOnly?: boolean;
}

export function isRootIdentity(user?: CurrentUser): boolean {
  return user?.root === true;
}

export function hasPermission(user: CurrentUser | undefined, permission?: string): boolean {
  if (!permission) return Boolean(user);
  if (isRootIdentity(user)) return true;
  return user?.permissions?.includes(permission) === true;
}

export function canAccessRoute(
  user: CurrentUser | undefined,
  route?: RouteAccessMetadata,
): boolean {
  if (!user) return false;
  if (route?.rootOnly && !isRootIdentity(user)) return false;
  return hasPermission(user, route?.permission);
}

export function flattenAuthorizedMenu(items: AuthorizedMenuItem[]): AuthorizedMenuItem[] {
  return items.flatMap((item) => [item, ...flattenAuthorizedMenu(item.children ?? [])]);
}
