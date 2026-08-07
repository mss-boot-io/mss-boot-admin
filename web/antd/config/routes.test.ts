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
});
