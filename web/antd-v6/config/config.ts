import { defineBusinessAdmin } from '@mss-boot-io/admin-web/business';
import { defineConfig } from '@umijs/max';
import customRoutes from './routes.custom';
import businessRoutes from './routes.generated';

export default defineConfig(
  defineBusinessAdmin({
    businessRoutes: [...businessRoutes, ...customRoutes],
    routeRegistrations: './src/route-registrations.ts',
    title: 'mss-boot-io',
    useUtoopack: true,
  }),
);
