const { existsSync, readFileSync } = require('node:fs');
const { resolve } = require('node:path');
const RuntimeStatsPlugin = require('./runtime-stats-plugin.cjs');
const { createAdminRoutes } = require('./core-routes.cjs');

const packageRoot = resolve(__dirname, '..');
const packageManifest = require('../package.json');
const emptyRegistrations = resolve(__dirname, 'empty-route-registrations.ts');
const environment = process.env.UMI_ENV || 'dev';
const browserQualification = process.env.MSS_V6_E2E === '1';

function logoDataURL() {
  const logo = readFileSync(resolve(packageRoot, 'public/logo.svg'), 'utf8');
  return `data:image/svg+xml,${encodeURIComponent(logo)}`;
}

function proxyConfig(apiTarget) {
  if (environment === 'release') return {};
  return {
    '/admin/': {
      target: apiTarget,
      changeOrigin: false,
      secure: false,
    },
  };
}

function validateBusinessAdminOptions(options) {
  if (options === null || typeof options !== 'object' || Array.isArray(options)) {
    throw new TypeError('Admin Web options must be an object.');
  }
  if ('routes' in options) {
    throw new Error(
      'Admin core routes are immutable; extend the complete application with businessRoutes instead of routes.',
    );
  }
  if (options.businessRoutes !== undefined && !Array.isArray(options.businessRoutes)) {
    throw new TypeError('Admin Web businessRoutes must be an array.');
  }
}

function defineBusinessAdmin(options = {}) {
  validateBusinessAdminOptions(options);
  const title = options.title || 'mss-boot-io';
  const logo = logoDataURL();
  const tailwindStyles = require.resolve('tailwindcss/index.css');
  const tailwindPostCSS = require.resolve('@tailwindcss/postcss');
  const apiTarget =
    options.apiTarget || process.env.MSS_ADMIN_API_TARGET || 'http://127.0.0.1:8080';
  const routeRegistrations = options.routeRegistrations
    ? resolve(process.cwd(), options.routeRegistrations)
    : emptyRegistrations;
  const routes = createAdminRoutes({
    businessRoutes: options.businessRoutes || [],
    pagesRoot: resolve(packageRoot, 'src/pages'),
  });
  const hasPostCSSConfig = ['postcss.config.cjs', 'postcss.config.js', 'postcss.config.mjs'].some(
    (name) => existsSync(resolve(process.cwd(), name)),
  );
  const useUtoopack = options.useUtoopack === true && environment !== 'release';

  return {
    presets: [require.resolve('./mssAdminPreset.cjs')],
    mssAdmin: {
      packageRoot,
      routeRegistrations,
    },
    alias: {
      '@mss-admin-core': resolve(packageRoot, 'src'),
      '@mss-admin-business/routes': routeRegistrations,
      '@root': packageRoot,
      tailwindcss: tailwindStyles,
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
      __APP_VERSION__: packageManifest.version,
      __ANTD_VERSION__: require('antd/package.json').version,
    },
    exportStatic: {},
    esbuildMinifyIIFE: true,
    extraBabelIncludes: [resolve(packageRoot, 'src')],
    extraPostCSSPlugins: hasPostCSSConfig ? [] : [require('@tailwindcss/postcss')],
    favicons: [logo],
    fastRefresh: true,
    hash: true,
    history: { type: 'browser' },
    initialState: {},
    jsMinifier: environment === 'release' ? 'terser' : 'esbuild',
    locale: {
      antd: true,
      baseNavigator: true,
      default: 'zh-CN',
      title: true,
    },
    layout: {
      locale: true,
      navTheme: 'realDark',
      colorPrimary: '#1677ff',
      layout: 'mix',
      contentWidth: 'Fluid',
      fixedHeader: false,
      fixSiderbar: true,
      colorWeak: false,
      title,
      logo,
      splitMenus: false,
    },
    manifest: {},
    mfsu: false,
    model: {},
    moment2dayjs: {
      preset: 'antd',
      plugins: ['duration', 'relativeTime'],
    },
    npmClient: 'pnpm',
    polyfill: { imports: [] },
    proxy: proxyConfig(apiTarget),
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
    title,
    utoopack: useUtoopack
      ? {
          persistentCaching: !browserQualification,
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
              d: resolve(packageRoot, 'src/shims/d.cjs'),
              'd/auto-bind': require.resolve('d/auto-bind'),
              tailwindcss: tailwindStyles,
            },
          },
          styles: {
            postcss: {
              plugins: {
                [tailwindPostCSS]: {},
              },
            },
          },
        }
      : false,
  };
}

module.exports = {
  defineBusinessAdmin,
};
