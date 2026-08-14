import type { AuthorizedMenuItem } from '@/shared/auth/types';

export interface RouteRegistration {
  path: string;
  permission?: string;
  rootOnly?: boolean;
}

export const routeRegistry = new Map<string, RouteRegistration>([
  ['/workplace', { path: '/workplace' }],
  ['/migration', { path: '/migration' }],
]);

export function retainRegisteredMenu(items: AuthorizedMenuItem[]): AuthorizedMenuItem[] {
  return items.flatMap((item) => {
    const children = retainRegisteredMenu(item.children ?? []);
    const registered = item.path ? routeRegistry.has(item.path) : false;
    if (!registered && children.length === 0) return [];
    return [{ ...item, children: children.length > 0 ? children : undefined }];
  });
}
