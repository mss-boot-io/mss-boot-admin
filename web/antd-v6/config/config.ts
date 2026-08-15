import { join } from 'node:path';
import { defineConfig } from '@umijs/max';
import defaultSettings from './defaultSettings';
import proxy from './proxy';
import routes from './routes';

const environment = process.env.UMI_ENV ?? 'dev';

export default defineConfig({
  alias: {
    '@root': join(__dirname, '..'),
  },
  antd: {
    appConfig: {},
    configProvider: {
      componentSize: 'middle',
      focusOutline: true,
      variant: 'filled',
      theme: {
        cssVar: { prefix: 'mss' },
        token: {
          colorPrimary: '#1677ff',
          borderRadius: 8,
          fontFamily:
            "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
        },
      },
    },
  },
  access: {},
  define: {
    __APP_VERSION__: require('../package.json').version,
    __ANTD_VERSION__: require('antd/package.json').version,
  },
  exportStatic: {},
  fastRefresh: true,
  hash: true,
  history: { type: 'browser' },
  initialState: {},
  locale: {
    antd: true,
    baseNavigator: true,
    default: 'zh-CN',
    title: true,
  },
  layout: {
    locale: true,
    ...defaultSettings,
  },
  manifest: {},
  model: {},
  npmClient: 'pnpm',
  plugins: ['@umijs/max-plugin-openapi'],
  proxy: proxy[environment as keyof typeof proxy],
  // The bootstrap path must use the same cache as page hooks. Disable the
  // plugin-created private client and provide the repository QueryClient once
  // from app.tsx, where logout and permission invalidation can also reach it.
  reactQuery: { queryClient: false },
  request: {},
  routePrefetch: {},
  routes,
  title: 'MSS Admin',
  utoopack: {},
});
