import { getMenuAuthorize } from '@/services/admin/menu';
import {
  isRootIdentity,
  type AuthorizationIdentity,
} from '@/utils/authorization';

type AuthorizedMenuNode = API.Menu & { children?: AuthorizedMenuNode[] };

export const ROOT_ONLY_MENU_PATHS = [
  '/system-config',
  '/security/online-sessions',
] as const;

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
) => (isRootIdentity(identity) ? menu : filterRootOnlyNodes(menu));

/**
 * Keeps ProLayout's runtime menu source explicit and independently testable.
 * The backend response remains authoritative except for the temporary root-only
 * hardening boundary, which also protects deployments with legacy Casbin rows.
 */
export const requestAuthorizedMenu = async (identity?: AuthorizationIdentity) =>
  filterAuthorizedMenuByIdentity(await getMenuAuthorize(), identity);

export default requestAuthorizedMenu;
