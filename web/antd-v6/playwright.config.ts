import { defineConfig, devices } from '@playwright/test';

const webServers = [];
const qualificationBaseURL = process.env.MSS_V6_BASE_URL ?? 'http://127.0.0.1:18001';

if (!process.env.MSS_V6_EXTERNAL_BACKEND) {
  webServers.push({
    command: 'bash scripts/start-e2e-backend.sh',
    url: 'http://127.0.0.1:18080/healthz',
    reuseExistingServer: false,
    timeout: 180_000,
  });
}

if (!process.env.MSS_V6_EXTERNAL_SERVER) {
  webServers.push({
    command: 'MSS_ADMIN_API_TARGET=http://127.0.0.1:18080 corepack pnpm@10.34.5 start:e2e',
    url: `${qualificationBaseURL}/admin/api/languages/public`,
    reuseExistingServer: false,
    timeout: 120_000,
  });
}

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  // The qualification backend intentionally uses one isolated SQLite file.
  // Serial workers keep browser projects deterministic while still exercising
  // the same HTTP and session boundaries as a deployed database.
  workers: 1,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: qualificationBaseURL,
    locale: 'en-US',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium-desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'chromium-mobile', use: { ...devices['Pixel 7'] } },
  ],
  webServer: webServers.length > 0 ? webServers : undefined,
});
