import routes from './routes';
import { getMenuAuthorize } from '@/services/admin/menu';
import {
  clearAuthorizedMenuRequestCache,
  requestAuthorizedMenu,
} from '@/utils/requestAuthorizedMenu';

jest.mock('@/services/admin/menu', () => ({
  getMenuAuthorize: jest.fn(),
}));

const mockGetMenuAuthorize = getMenuAuthorize as jest.Mock;

type RouteNode = {
  path?: string;
  component?: string;
  redirect?: string;
  access?: string;
  permission?: string;
  rootOnly?: boolean;
  routes?: RouteNode[];
};

function flattenRoutes(nodes: RouteNode[]): RouteNode[] {
  return nodes.flatMap((node) => [node, ...flattenRoutes(node.routes ?? [])]);
}

beforeEach(() => {
  jest.clearAllMocks();
  clearAuthorizedMenuRequestCache();
});

describe('static route inventory', () => {
  it('does not expose retired runtime developer tools', () => {
    const inventory = flattenRoutes(routes as RouteNode[]);
    const retiredPaths = inventory
      .map(({ path }) => path)
      .filter(
        (path): path is string =>
          typeof path === 'string' &&
          (path === '/generator' ||
            path === '/model' ||
            path.startsWith('/model/') ||
            path === '/field' ||
            path.startsWith('/field/') ||
            path === '/virtual' ||
            path.startsWith('/virtual/')),
      );
    const retiredComponents = inventory
      .map(({ component }) => component)
      .filter((component) =>
        ['./Generator', './Model', './Field', './Virtual'].includes(component ?? ''),
      );

    expect(retiredPaths).toEqual([]);
    expect(retiredComponents).toEqual([]);
  });

  it('keeps normal product routes and the not-found fallback', () => {
    const paths = flattenRoutes(routes as RouteNode[]).map(({ path }) => path);

    expect(paths).toEqual(
      expect.arrayContaining(['/welcome', '/analysis', '/workplace', '/users', '/security', '*']),
    );
  });

  it('uses workplace as the canonical dashboard route', () => {
    const inventory = flattenRoutes(routes as RouteNode[]);
    const route = (path: string) => inventory.find((node) => node.path === path);

    expect(route('/workplace')).toMatchObject({
      component: './Welcome',
      access: 'canAccessRoute',
      permission: '/welcome',
    });
    expect(route('/welcome')).toMatchObject({ redirect: '/workplace' });
    expect(route('/analysis')).toMatchObject({ redirect: '/workplace' });
    expect(route('/welcome')?.component).toBeUndefined();
    expect(route('/analysis')?.component).toBeUndefined();
  });

  it('protects direct static routes with menu and component permission markers', () => {
    const inventory = flattenRoutes(routes as RouteNode[]);
    const route = (path: string) => inventory.find((node) => node.path === path);

    expect(route('/workplace')).toMatchObject({
      access: 'canAccessRoute',
      permission: '/welcome',
    });
    expect(route('/role/create')).toMatchObject({
      access: 'canAccessRoute',
      rootOnly: true,
    });
    expect(route('/role/:id')).toMatchObject({ access: 'canAccessRoute', rootOnly: true });
    expect(route('/departments/create')).toMatchObject({
      access: 'canAccessRoute',
      rootOnly: true,
    });
    expect(route('/departments/:id')).toMatchObject({
      access: 'canAccessRoute',
      rootOnly: true,
    });
    expect(route('/posts/create')).toMatchObject({
      access: 'canAccessRoute',
      rootOnly: true,
    });
    expect(route('/posts/:id')).toMatchObject({
      access: 'canAccessRoute',
      rootOnly: true,
    });
    expect(route('/menu/create')).toMatchObject({ access: 'canAccessRoute', rootOnly: true });
    expect(route('/menu/:id')).toMatchObject({ access: 'canAccessRoute', rootOnly: true });
    const systemConfigRoutes = inventory.filter((node) => node.path === '/system-config');
    expect(systemConfigRoutes).toHaveLength(2);
    systemConfigRoutes.forEach((node) =>
      expect(node).toMatchObject({ access: 'canAccessRoute', rootOnly: true }),
    );
    expect(route('/system-config/create')).toMatchObject({
      access: 'canAccessRoute',
      rootOnly: true,
    });
    expect(route('/system-config/:id')).toMatchObject({
      access: 'canAccessRoute',
      rootOnly: true,
    });
    expect(route('/users/control/create')).toMatchObject({
      access: 'canAccessRoute',
      rootOnly: true,
    });
    expect(route('/users/control/:id')).toMatchObject({
      access: 'canAccessRoute',
      rootOnly: true,
    });
    expect(route('/users/password-reset/:id')).toMatchObject({
      access: 'canAccessRoute',
      permission: '/users/password-reset',
    });
    expect(route('/security/online-sessions')).toMatchObject({
      access: 'canAccessRoute',
      rootOnly: true,
    });
  });

  it('keeps the complete anonymous route manifest routable', () => {
    const paths = flattenRoutes(routes as RouteNode[]).map(({ path }) => path);

    expect(paths).toEqual(
      expect.arrayContaining([
        '/user/login',
        '/user/forget',
        '/user/register',
        '/user/callback/:provider',
      ]),
    );
  });
});

describe('runtime authorized menu', () => {
  it('returns the backend menu tree unchanged so unrelated dynamic menus remain available', async () => {
    const menuTree = [
      { path: '/workplace' },
      { path: '/custom', children: [{ path: '/custom/report' }] },
    ];
    mockGetMenuAuthorize.mockResolvedValueOnce(menuTree);

    await expect(requestAuthorizedMenu()).resolves.toBe(menuTree);
    expect(mockGetMenuAuthorize).toHaveBeenCalledTimes(1);
    expect(mockGetMenuAuthorize).toHaveBeenCalledWith();
  });

  it('keeps root-only management pages for root without reshaping the backend tree', async () => {
    const menuTree = [
      { path: '/system-config', type: 'MENU' },
      {
        path: '/security',
        type: 'DIRECTORY',
        children: [{ path: '/security/online-sessions', type: 'MENU' }],
      },
    ];
    mockGetMenuAuthorize.mockResolvedValueOnce(menuTree);

    await expect(requestAuthorizedMenu({ role: { root: true } })).resolves.toBe(menuTree);
  });

  it('removes legacy root-only entries and newly empty directories for non-root', async () => {
    const menuTree = [
      { path: '/welcome', type: 'MENU' },
      { path: '/system-config/', type: 'MENU' },
      { path: '/system-config/create?source=legacy', type: 'MENU' },
      {
        path: '/security',
        type: 'DIRECTORY',
        children: [{ path: '/security/online-sessions', type: 'MENU' }],
      },
      {
        path: '/custom',
        type: 'DIRECTORY',
        children: [{ path: '/custom/report', type: 'MENU' }],
      },
    ];
    mockGetMenuAuthorize.mockResolvedValueOnce(menuTree);

    await expect(
      requestAuthorizedMenu({
        role: { root: false },
        permissions: {
          '/system-config': true,
          '/security/online-sessions': true,
        },
      }),
    ).resolves.toEqual([
      { path: '/workplace', type: 'MENU' },
      {
        path: '/custom',
        type: 'DIRECTORY',
        children: [{ path: '/custom/report', type: 'MENU' }],
      },
    ]);
    expect(menuTree[3].children).toHaveLength(1);
  });

  it('projects legacy dashboard menu aliases without mutating other metadata', async () => {
    const menuTree = [
      { path: '/welcome/?source=legacy', name: 'welcome', type: 'MENU', sort: 1 },
      { path: '/analysis#overview', name: 'analysis', type: 'MENU', sort: 2 },
      { path: '/custom/report', name: 'custom.report', type: 'MENU', sort: 3 },
    ];
    mockGetMenuAuthorize.mockResolvedValueOnce(menuTree);

    await expect(requestAuthorizedMenu({ role: { root: true } }, 4)).resolves.toEqual([
      { path: '/workplace', name: 'welcome', type: 'MENU', sort: 1 },
      { path: '/workplace', name: 'analysis', type: 'MENU', sort: 2 },
      { path: '/custom/report', name: 'custom.report', type: 'MENU', sort: 3 },
    ]);
    expect(menuTree.map((item) => item.path)).toEqual([
      '/welcome/?source=legacy',
      '/analysis#overview',
      '/custom/report',
    ]);
  });
});
