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
  favicons: ['/logo.svg'],
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
  // Max still carries transitional Moment consumers. Keep one Day.js runtime
  // for Ant Design and retained duration/relative-time behavior instead of
  // shipping Moment plus every locale in the independent V6 application.
  moment2dayjs: {
    preset: 'antd',
    plugins: ['duration', 'relativeTime'],
  },
  npmClient: 'pnpm',
  plugins: ['@umijs/max-plugin-openapi'],
  // Umi defaults to Chrome 80 and injects a broad core-js bundle. V6 only
  // supports the documented evergreen baseline, so compile for that contract
  // and let browsers provide the platform instead of silently widening it.
  polyfill: { imports: [] },
  proxy: proxy[environment as keyof typeof proxy],
  // The bootstrap path must use the same cache as page hooks. Disable the
  // plugin-created private client and provide the repository QueryClient once
  // from app.tsx, where logout and permission invalidation can also reach it.
  reactQuery: { queryClient: false },
  request: {},
  routePrefetch: {},
  routes,
  targets: {
    chrome: 120,
    edge: 120,
    firefox: 121,
    safari: '17.4',
  },
  title: 'mss-boot-io',
  utoopack: {},
});
