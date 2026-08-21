import { defineBusinessAdmin } from '@mss-boot-io/admin-web/business';
import { defineConfig } from '@umijs/max';
import businessRoutes from './routes.generated';

export default defineConfig(
  defineBusinessAdmin({
    businessRoutes,
    routeRegistrations: './src/generated/routes.ts',
    title: 'mss-boot-io',
    useUtoopack: true,
  }),
);
