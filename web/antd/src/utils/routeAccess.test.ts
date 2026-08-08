import {
  isPublicRoute,
  matchesRoutePattern,
  PUBLIC_ROUTE_PATHS,
  resolveCrudRouteID,
} from './routeAccess';

describe('routeAccess', () => {
  it('keeps every anonymous identity route in one manifest', () => {
    expect(isPublicRoute(PUBLIC_ROUTE_PATHS.login)).toBe(true);
    expect(isPublicRoute(PUBLIC_ROUTE_PATHS.forget)).toBe(true);
    expect(isPublicRoute(PUBLIC_ROUTE_PATHS.register)).toBe(true);
    expect(isPublicRoute('/user/callback/github')).toBe(true);
    expect(isPublicRoute('/user/callback/lark')).toBe(true);
  });

  it('matches exactly one non-empty segment for route parameters', () => {
    expect(matchesRoutePattern('/user/callback/:provider', '/user/callback/github')).toBe(true);
    expect(matchesRoutePattern('/user/callback/:provider', '/user/callback')).toBe(false);
    expect(matchesRoutePattern('/user/callback/:provider', '/user/callback/github/extra')).toBe(
      false,
    );
  });

  it('does not make management or unknown routes public', () => {
    expect(isPublicRoute('/role')).toBe(false);
    expect(isPublicRoute('/departments/create')).toBe(false);
    expect(isPublicRoute('/does-not-exist')).toBe(false);
  });

  it('resolves static create routes without weakening dynamic route IDs', () => {
    const createPaths = [
      '/users/control/create',
      '/role/create',
      '/departments/create',
      '/posts/create',
      '/task/create',
      '/language/create',
      '/menu/create',
      '/system-config/create',
      '/option/create',
    ];
    createPaths.forEach((createPath) => {
      expect(resolveCrudRouteID(undefined, createPath, createPath)).toBe('create');
    });
    expect(resolveCrudRouteID(undefined, '/role/create/', '/role/create')).toBe('create');
    expect(resolveCrudRouteID('role-1', '/role/role-1', '/role/create')).toBe('role-1');
    expect(resolveCrudRouteID(undefined, '/role', '/role/create')).toBeUndefined();
    expect(resolveCrudRouteID(undefined, '/role/create-copy', '/role/create')).toBeUndefined();
  });
});
