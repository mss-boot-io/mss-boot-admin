import type { AdminBusinessRoute } from '@mss-boot-io/admin-web/business';
import generatedRoutes from './routes.generated';

const { createAdminRoutes } = require('../package/core-routes.cjs') as {
  createAdminRoutes: (options?: { businessRoutes?: AdminBusinessRoute[] }) => AdminBusinessRoute[];
};

export default createAdminRoutes({ businessRoutes: generatedRoutes });
