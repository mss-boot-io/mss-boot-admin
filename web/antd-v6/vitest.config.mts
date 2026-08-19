import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@mss-admin-core': fileURLToPath(new URL('./src', import.meta.url)),
      '@mss-admin-business/routes': fileURLToPath(
        new URL('./src/generated/routes.ts', import.meta.url),
      ),
    },
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: ['./tests/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}', 'tests/**/*.test.{ts,tsx}'],
    maxWorkers: 2,
    clearMocks: true,
    restoreMocks: true,
  },
});
