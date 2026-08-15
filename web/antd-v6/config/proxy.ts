import type { IApi } from '@umijs/max';

type ProxyConfig = Parameters<IApi['modifyConfig']>[0] extends never
  ? never
  : Record<string, unknown>;

const localTarget = process.env.MSS_ADMIN_API_TARGET ?? 'http://127.0.0.1:8080';

const proxy = {
  dev: {
    '/admin/': {
      target: localTarget,
      // Preserve the browser Origin so the backend's exact CSRF allowlist is
      // exercised in development and E2E just as it is behind production Nginx.
      changeOrigin: false,
      secure: false,
    },
  },
  local: {
    '/admin/': {
      target: localTarget,
      changeOrigin: false,
      secure: false,
    },
  },
  release: {},
} satisfies Record<string, ProxyConfig>;

export default proxy;
