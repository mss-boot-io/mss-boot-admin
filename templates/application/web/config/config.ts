import { defineBusinessAdmin } from '__MSS_DISTRIBUTION_FRONTEND_PACKAGE__/business';
import businessRoutes from './business-routes';

export default defineBusinessAdmin({
  businessRoutes,
  routeRegistrations: './src/route-registrations.ts',
  useUtoopack: true,
});
