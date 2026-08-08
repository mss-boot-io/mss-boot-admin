import routes from './routes';
import { getMenuAuthorize } from '@/services/admin/menu';
import { requestAuthorizedMenu } from '@/utils/requestAuthorizedMenu';

jest.mock('@/services/admin/menu', () => ({
  getMenuAuthorize: jest.fn(),
}));

const mockGetMenuAuthorize = getMenuAuthorize as jest.Mock;

type RouteNode = {
  path?: string;
  component?: string;
  access?: string;
  permission?: string;
  rootOnly?: boolean;
  routes?: RouteNode[];
};

function flattenRoutes(nodes: RouteNode[]): RouteNode[] {
  return nodes.flatMap((node) => [node, ...flattenRoutes(node.routes ?? [])]);
}

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

    expect(paths).toEqual(expect.arrayContaining(['/welcome', '/users', '/security', '*']));
  });

  it('protects direct static routes with menu and component permission markers', () => {
    const inventory = flattenRoutes(routes as RouteNode[]);
    const route = (path: string) => inventory.find((node) => node.path === path);

    expect(route('/welcome')).toMatchObject({
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
      { path: '/welcome' },
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
      { path: '/welcome', type: 'MENU' },
      {
        path: '/custom',
        type: 'DIRECTORY',
        children: [{ path: '/custom/report', type: 'MENU' }],
      },
    ]);
    expect(menuTree[3].children).toHaveLength(1);
  });
});
