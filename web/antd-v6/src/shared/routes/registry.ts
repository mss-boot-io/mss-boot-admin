import { canAccessRoute } from '@/shared/auth/access';
import type { AuthorizedMenuItem, CurrentUser } from '@/shared/auth/types';

export interface RouteRegistration {
  path: string;
  serverPaths: readonly string[];
  menuName: string;
  permission?: string;
  rootOnly?: boolean;
}

const registrations: readonly RouteRegistration[] = [
  {
    path: '/workplace',
    serverPaths: ['/welcome'],
    menuName: 'workplace',
    permission: '/welcome',
  },
  {
    path: '/app-config',
    serverPaths: ['/app-config'],
    menuName: 'app-config',
    permission: '/app-config',
  },
  {
    path: '/language',
    serverPaths: ['/language'],
    menuName: 'language',
    permission: '/language',
  },
  {
    path: '/option',
    serverPaths: ['/option'],
    menuName: 'option',
    permission: '/option',
  },
  {
    path: '/security/online-sessions',
    serverPaths: ['/security/online-sessions'],
    menuName: 'online-sessions',
    rootOnly: true,
  },
];

export const routeRegistry = new Map(registrations.map((entry) => [entry.path, entry]));

const serverRouteRegistry = new Map(
  registrations.flatMap((entry) => entry.serverPaths.map((path) => [path, entry] as const)),
);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function optionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined;
}

function optionalBoolean(value: unknown): boolean | undefined {
  return typeof value === 'boolean' ? value : undefined;
}

function safeMetadata(item: Record<string, unknown>): AuthorizedMenuItem {
  return {
    id: optionalString(item.id),
    key: optionalString(item.id),
    name: optionalString(item.name),
    title: optionalString(item.title),
    icon: optionalString(item.icon),
    type: optionalString(item.type),
    hideChildrenInMenu: optionalBoolean(item.hideChildrenInMenu),
    hideInMenu: optionalBoolean(item.hideInMenu),
    hideInBreadcrumb: optionalBoolean(item.hideInBreadcrumb),
    flatMenu: optionalBoolean(item.flatMenu),
  };
}

/**
 * Intersect backend authorization metadata with executable routes compiled
 * into this application. Unknown paths can survive only as non-navigable
 * directory labels when they contain an authorized registered descendant.
 * Backend component strings and external paths are never copied to the result.
 */
export function retainRegisteredMenu(value: unknown, user?: CurrentUser): AuthorizedMenuItem[] {
  if (!Array.isArray(value)) return [];

  const result: AuthorizedMenuItem[] = [];
  for (const candidate of value) {
    if (!isRecord(candidate)) continue;
    const sourcePath = optionalString(candidate.path);
    const registration = sourcePath ? serverRouteRegistry.get(sourcePath) : undefined;
    const rawChildren = Array.isArray(candidate.children) ? candidate.children : [];
    const children = retainRegisteredMenu(rawChildren, user);

    if (registration && canAccessRoute(user, registration)) {
      result.push({
        ...safeMetadata(candidate),
        key: optionalString(candidate.id) ?? registration.path,
        name: registration.menuName,
        path: registration.path,
        sourcePath,
        permission: registration.permission,
        rootOnly: registration.rootOnly,
        children: children.length > 0 ? children : undefined,
      });
      continue;
    }

    if (children.length > 0) {
      result.push({
        ...safeMetadata(candidate),
        key: optionalString(candidate.id) ?? `directory:${sourcePath ?? 'anonymous'}`,
        sourcePath,
        path: undefined,
        children,
      });
    }
  }
  return result;
}
