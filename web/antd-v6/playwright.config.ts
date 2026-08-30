import { defineConfig, devices } from '@playwright/test';

const webServers = [];
const qualificationBaseURL = process.env.MSS_V6_BASE_URL ?? 'http://127.0.0.1:18001';
const qualificationBackendOrigin = process.env.MSS_V6_BACKEND_ORIGIN ?? 'http://127.0.0.1:18080';
const inheritedEnvironment = Object.fromEntries(
  Object.entries(process.env).filter(
    (entry): entry is [string, string] => typeof entry[1] === 'string',
  ),
);

function originPort(name: string, origin: string): string {
  const parsed = new URL(origin);
  const port = parsed.port || (parsed.protocol === 'https:' ? '443' : '80');
  const numeric = Number(port);
  if (!Number.isInteger(numeric) || numeric < 1 || numeric > 65_535) {
    throw new Error(`${name} must contain a valid TCP port`);
  }
  return port;
}

const qualificationBackendPort = originPort('MSS_V6_BACKEND_ORIGIN', qualificationBackendOrigin);
const qualificationWebPort = originPort('MSS_V6_BASE_URL', qualificationBaseURL);

if (!process.env.MSS_V6_EXTERNAL_BACKEND) {
  webServers.push({
    command: 'bash scripts/start-e2e-backend.sh',
    env: {
      ...inheritedEnvironment,
      MSS_V6_BACKEND_PORT: qualificationBackendPort,
      MSS_V6_WEB_PORT: qualificationWebPort,
      MSS_E2E_DOMAIN: `127.0.0.1:${qualificationWebPort}`,
    },
    url: `${qualificationBackendOrigin}/healthz`,
    reuseExistingServer: false,
    timeout: 180_000,
  });
}

if (!process.env.MSS_V6_EXTERNAL_SERVER) {
  webServers.push({
    command: 'corepack pnpm@10.34.5 exec max dev',
    env: {
      ...inheritedEnvironment,
      MSS_ADMIN_API_TARGET: qualificationBackendOrigin,
      MSS_V6_E2E: '1',
      PORT: qualificationWebPort,
      REACT_APP_ENV: 'dev',
      UMI_ENV: 'dev',
      MOCK: 'none',
    },
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
