import { join } from 'node:path';
import { defineConfig } from '@umijs/max';
import defaultSettings from './defaultSettings';
import proxy from './proxy';
import RuntimeStatsPlugin from './RuntimeStatsPlugin';
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
  // Umi recommends granular shared chunks for application projects. Keep the
  // React framework runtime and large reusable libraries independently cached
  // instead of forcing every route through one oversized async payload.
  codeSplitting: { jsStrategy: 'granularChunks' },
  chainWebpack(memo) {
    if (environment !== 'release') return memo;

    memo.plugin('runtime-graph-stats').use(RuntimeStatsPlugin);
    memo.optimization.runtimeChunk({ name: 'runtime' });
    memo.optimization.splitChunks({
      cacheGroups: {
        antDesignRuntime: {
          chunks: 'initial',
          enforce: true,
          name: 'ant-design-runtime',
          priority: 35,
          test: /[\\/]node_modules[\\/]@ant-design[\\/]/,
        },
        antdRuntime: {
          chunks: 'initial',
          enforce: true,
          name: 'antd-runtime',
          priority: 35,
          test: /[\\/]node_modules[\\/]antd[\\/]/,
        },
        applicationShell: {
          chunks: 'initial',
          enforce: true,
          name: 'application-shell',
          priority: 34,
          test: /[\\/]src[\\/](?:modules[\\/]language|shared)[\\/]/,
        },
        proComponents: {
          chunks: 'all',
          enforce: true,
          name: 'pro-components',
          priority: 36,
          test: /[\\/]node_modules[\\/]@ant-design[\\/]pro-/,
        },
        queryRuntime: {
          chunks: 'initial',
          enforce: true,
          name: 'query-runtime',
          priority: 32,
          test: /[\\/]node_modules[\\/](?:@tanstack|axios|ahooks)[\\/]/,
        },
        umiRuntime: {
          chunks: 'initial',
          enforce: true,
          name: 'umi-runtime',
          priority: 33,
          test: /(?:[\\/]src[\\/]\.umi-production[\\/]|[\\/]node_modules[\\/](?:@umijs|umi)[\\/])/,
        },
        rcRuntime: {
          chunks: 'initial',
          enforce: true,
          name: 'rc-runtime',
          priority: 35,
          test: /[\\/]node_modules[\\/](?:@rc-component|rc-[^\\/]+)[\\/]/,
        },
        vendors: {
          chunks: 'initial',
          enforce: true,
          name: 'vendors',
          priority: 20,
          test: /[\\/]node_modules[\\/]/,
        },
      },
    });
    return memo;
  },
  access: {},
  define: {
    __APP_VERSION__: require('../package.json').version,
    __ANTD_VERSION__: require('antd/package.json').version,
  },
  exportStatic: {},
  esbuildMinifyIIFE: true,
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
  // Utoopack keeps local development fast, while its splitChunks support is
  // still incomplete in 1.5.5. Use Umi's mature production bundler for the
  // release artifact so granularChunks can enforce cache and transfer budgets.
  utoopack:
    environment === 'release'
      ? false
      : {
          optimization: {
            splitChunks: {
              js: {
                maxChunkCountPerGroup: 40,
                maxMergeChunkSize: 200_000,
                minChunkSize: 50_000,
              },
            },
          },
          resolve: {
            alias: {
              // Utoopack 1.5.5 expands this legacy CJS leaf through the complete
              // es5-ext/string barrel. Keep the descriptor API used by Umi's legacy
              // event emitter without shipping that obsolete barrel to browsers.
              d: join(__dirname, '../src/shims/d.cjs'),
              'd/auto-bind': require.resolve('d/auto-bind'),
            },
          },
        },
});
