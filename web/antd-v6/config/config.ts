import { defineBusinessAdmin } from '@mss-boot-io/admin-web/business';
import { defineConfig } from '@umijs/max';
import routes from './routes';

export default defineConfig(
  defineBusinessAdmin({
    routeRegistrations: './src/generated/routes.ts',
    routes,
    title: 'mss-boot-io',
    useUtoopack: true,
  }),
);
