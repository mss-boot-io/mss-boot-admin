export interface AdminBusinessRoute {
  path?: string;
  component?: string;
  redirect?: string;
  routes?: AdminBusinessRoute[];
  [key: string]: unknown;
}

export interface BusinessAdminOptions {
  title?: string;
  apiTarget?: string;
  businessRoutes?: AdminBusinessRoute[];
  routes?: AdminBusinessRoute[];
  routeRegistrations?: string;
  useUtoopack?: boolean;
}

export declare function defineBusinessAdmin(
  options?: BusinessAdminOptions,
): Record<string, unknown>;
