import { getMenuAuthorize } from '@/services/admin/menu';
import {
  isRootIdentity,
  type AuthorizationIdentity,
} from '@/utils/authorization';

export type AuthorizedMenuNode = API.Menu & { children?: AuthorizedMenuNode[] };

type AuthorizedMenuRequestCacheEntry = {
  permissionRefreshVersion: number;
  request: Promise<AuthorizedMenuNode[]>;
};

const authorizedMenuRequestCache = new WeakMap<object, AuthorizedMenuRequestCacheEntry>();
let anonymousAuthorizedMenuRequestCache: AuthorizedMenuRequestCacheEntry | undefined;

export const ROOT_ONLY_MENU_PATHS = [
  '/system-config',
  '/security/online-sessions',
] as const;

export const LEGACY_WORKPLACE_MENU_PATHS = ['/welcome', '/analysis'] as const;

const normalizeMenuPath = (path?: string) => {
  if (!path) return '';
  const pathname = path.split(/[?#]/, 1)[0].replace(/\/+$/, '');
  return pathname || '/';
};

const isRootOnlyMenuPath = (path?: string) => {
  const pathname = normalizeMenuPath(path);
  return ROOT_ONLY_MENU_PATHS.some(
    (rootPath) => pathname === rootPath || pathname.startsWith(`${rootPath}/`),
  );
};

const canonicalizeWorkplaceMenuPath = (path?: string) => {
  const pathname = normalizeMenuPath(path);
  return LEGACY_WORKPLACE_MENU_PATHS.some((legacyPath) => pathname === legacyPath)
    ? '/workplace'
    : path;
};

export const projectLegacyWorkplaceMenuPaths = (
  nodes: AuthorizedMenuNode[],
): AuthorizedMenuNode[] => {
  let changed = false;
  const projected = nodes.map((node) => {
    const path = canonicalizeWorkplaceMenuPath(node.path);
    const children = node.children ? projectLegacyWorkplaceMenuPaths(node.children) : undefined;
    if (path === node.path && children === node.children) {
      return node;
    }

    changed = true;
    return {
      ...node,
      path,
      ...(node.children ? { children } : {}),
    };
  });

  return changed ? projected : nodes;
};

const filterRootOnlyNodes = (nodes: AuthorizedMenuNode[]): AuthorizedMenuNode[] => {
  let changed = false;
  const filtered: AuthorizedMenuNode[] = [];

  nodes.forEach((node) => {
    if (isRootOnlyMenuPath(node.path)) {
      changed = true;
      return;
    }

    let next = node;
    if (node.children) {
      const children = filterRootOnlyNodes(node.children);
      if (children !== node.children) {
        next = { ...node, children };
        changed = true;
      }
    }

    if (next.type === 'DIRECTORY' && (!next.children || next.children.length === 0)) {
      changed = true;
      return;
    }

    filtered.push(next);
  });

  return changed ? filtered : nodes;
};

export const filterAuthorizedMenuByIdentity = (
  menu: AuthorizedMenuNode[],
  identity?: AuthorizationIdentity,
) =>
  projectLegacyWorkplaceMenuPaths(
    isRootIdentity(identity) ? menu : filterRootOnlyNodes(menu),
  );

const normalizePermissionRefreshVersion = (permissionRefreshVersion?: number) =>
  Number.isFinite(permissionRefreshVersion) ? Number(permissionRefreshVersion) : 0;

const getCachedRequest = (
  identity: AuthorizationIdentity | undefined,
  permissionRefreshVersion: number,
) => {
  const entry = identity
    ? authorizedMenuRequestCache.get(identity)
    : anonymousAuthorizedMenuRequestCache;
  return entry?.permissionRefreshVersion === permissionRefreshVersion ? entry.request : undefined;
};

const setCachedRequest = (
  identity: AuthorizationIdentity | undefined,
  entry: AuthorizedMenuRequestCacheEntry,
) => {
  if (identity) {
    authorizedMenuRequestCache.set(identity, entry);
  } else {
    anonymousAuthorizedMenuRequestCache = entry;
  }
};

const deleteCachedRequest = (
  identity: AuthorizationIdentity | undefined,
  request: Promise<AuthorizedMenuNode[]>,
) => {
  const entry = identity
    ? authorizedMenuRequestCache.get(identity)
    : anonymousAuthorizedMenuRequestCache;
  if (entry?.request !== request) return;

  if (identity) {
    authorizedMenuRequestCache.delete(identity);
  } else {
    anonymousAuthorizedMenuRequestCache = undefined;
  }
};

export const clearAuthorizedMenuRequestCache = (identity?: AuthorizationIdentity) => {
  if (identity) {
    authorizedMenuRequestCache.delete(identity);
    return;
  }
  anonymousAuthorizedMenuRequestCache = undefined;
};

/**
 * Keeps ProLayout's runtime menu source explicit and independently testable.
 * The backend response remains authoritative except for the temporary root-only
 * hardening boundary, which also protects deployments with legacy Casbin rows.
 */
export const requestAuthorizedMenu = (
  identity?: AuthorizationIdentity,
  permissionRefreshVersion?: number,
) => {
  const normalizedVersion = normalizePermissionRefreshVersion(permissionRefreshVersion);
  const cached = getCachedRequest(identity, normalizedVersion);
  if (cached) return cached;

  let request: Promise<AuthorizedMenuNode[]>;
  request = getMenuAuthorize()
    .then((menu) => filterAuthorizedMenuByIdentity(menu, identity))
    .catch((error) => {
      deleteCachedRequest(identity, request);
      throw error;
    });
  setCachedRequest(identity, {
    permissionRefreshVersion: normalizedVersion,
    request,
  });
  return request;
};

export default requestAuthorizedMenu;
