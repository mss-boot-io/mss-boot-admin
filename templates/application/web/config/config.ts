import { defineBusinessAdmin } from '__MSS_DISTRIBUTION_FRONTEND_PACKAGE__/business';
import businessRoutes from './business-routes.generated';

export default defineBusinessAdmin({
  businessRoutes,
  routeRegistrations: './src/generated/routes.ts',
  useUtoopack: true,
});
