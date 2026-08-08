export const PUBLIC_ROUTE_PATHS = {
  login: '/user/login',
  forget: '/user/forget',
  register: '/user/register',
  callback: '/user/callback/:provider',
} as const;

// Legacy callback paths remain public during the compatibility window even
// though the canonical route is /user/callback/:provider.
export const PUBLIC_ROUTE_PATTERNS = [
  ...Object.values(PUBLIC_ROUTE_PATHS),
  '/user/github-callback',
  '/user/lark-callback',
] as const;

export const matchesRoutePattern = (pattern: string, pathname: string) => {
  const patternSegments = pattern.split('/').filter(Boolean);
  const pathSegments = pathname.split('/').filter(Boolean);
  if (patternSegments.length !== pathSegments.length) {
    return false;
  }
  return patternSegments.every(
    (segment, index) => segment.startsWith(':') || segment === pathSegments[index],
  );
};

export const isPublicRoute = (pathname: string) =>
  PUBLIC_ROUTE_PATTERNS.some((pattern) => matchesRoutePattern(pattern, pathname));

const normalizeRoutePath = (pathname: string) => {
  const normalized = pathname.replace(/\/+$/, '');
  return normalized || '/';
};

export const resolveCrudRouteID = (
  routeID: string | undefined,
  pathname: string,
  createPath: string,
) =>
  routeID ??
  (normalizeRoutePath(pathname) === normalizeRoutePath(createPath) ? 'create' : undefined);
