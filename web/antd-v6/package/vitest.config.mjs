import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';

const packageRoot = fileURLToPath(new URL('..', import.meta.url));
const projectRoot = process.cwd();
const managedRouteRegistrations = `${projectRoot}/src/route-registrations.ts`;
const generatedRouteRegistrations = `${projectRoot}/src/generated/routes.ts`;
const routeRegistrations = existsSync(managedRouteRegistrations)
  ? managedRouteRegistrations
  : generatedRouteRegistrations;

export default defineConfig({
  resolve: {
    alias: [
      {
        find: '@mss-admin-business/routes',
        replacement: routeRegistrations,
      },
      { find: '@mss-admin-core', replacement: `${packageRoot}/src` },
      { find: '@', replacement: `${projectRoot}/src` },
    ],
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    passWithNoTests: true,
    setupFiles: [`${packageRoot}/tests/setup.ts`],
    include: ['src/**/*.test.{ts,tsx}', 'tests/**/*.test.{ts,tsx}'],
    maxWorkers: 2,
    clearMocks: true,
    restoreMocks: true,
  },
});
