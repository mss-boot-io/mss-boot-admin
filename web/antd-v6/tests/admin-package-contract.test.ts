import { execFileSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { isAbsolute, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import manifest from '../package.json';

const packageManifest = manifest as typeof manifest & {
  private?: boolean;
  mssAdminDistribution?: {
    packageManager?: string;
    dependencyClasses?: {
      runtime?: string[];
      tooling?: string[];
    };
    buildOnlyDependencies?: Record<string, string[]>;
    runtimeOverrides?: Record<string, string>;
  };
};

interface Route {
  path?: string;
  component?: string;
  routes?: Route[];
}

const require = createRequire(import.meta.url);
const { createAdminRoutes } = require('../package/core-routes.cjs') as {
  createAdminRoutes: (options?: { businessRoutes?: Route[] }) => Route[];
};
const { defineBusinessAdmin } = require('../package/business.cjs') as {
  defineBusinessAdmin: (options?: Record<string, unknown>) => Record<string, unknown>;
};
const mssAdminPlugin = require('../package/mssAdminPlugin.cjs') as (
  api: Record<string, unknown>,
) => void;
const RuntimeStatsPlugin = require('../package/runtime-stats-plugin.cjs') as new () => {
  apply: (compiler: Record<string, unknown>) => void;
};

describe('Admin web package contract', () => {
  it('exposes only stable public entrypoints', () => {
    expect(Object.keys(packageManifest.exports).sort()).toEqual(
      [
        '.',
        './business',
        './package.json',
        './preset',
        './runtime',
        './runtime/access',
        './runtime/app',
        './runtime/presentation',
        './runtime/presentation/client',
        './runtime/locales/en-US',
        './runtime/locales/zh-CN',
        './styles',
        './testing',
      ].sort(),
    );
    expect(Object.keys(packageManifest.exports)).not.toContain('./src/*');
    expect(packageManifest.name).toBe('@mss-boot-io/admin-web');
    expect(packageManifest.private).not.toBe(true);
    expect(packageManifest.publishConfig).toEqual({
      access: 'public',
      registry: 'https://registry.npmjs.org',
    });
    expect(packageManifest.mssAdminDistribution).toMatchObject({
      packageManager: packageManifest.packageManager,
      runtimeOverrides: packageManifest.pnpm.overrides,
    });
    expect(Object.keys(packageManifest.mssAdminDistribution ?? {}).sort()).toEqual(
      ['buildOnlyDependencies', 'dependencyClasses', 'packageManager', 'runtimeOverrides'].sort(),
    );
    const runtimeDependencies =
      packageManifest.mssAdminDistribution?.dependencyClasses?.runtime ?? [];
    const toolingDependencies =
      packageManifest.mssAdminDistribution?.dependencyClasses?.tooling ?? [];
    expect([...runtimeDependencies, ...toolingDependencies].sort()).toEqual(
      Object.keys(packageManifest.dependencies).sort(),
    );
    expect(runtimeDependencies.filter((name) => toolingDependencies.includes(name))).toEqual([]);
    const buildOnlyDependencies = packageManifest.mssAdminDistribution?.buildOnlyDependencies ?? {};
    expect(Object.keys(buildOnlyDependencies)).toEqual(Object.keys(buildOnlyDependencies).sort());
    for (const versions of Object.values(buildOnlyDependencies)) {
      expect(versions).toEqual([...new Set(versions)].sort());
    }
    expect(packageManifest.dependencies.vite).toBe('8.2.2');

    const businessTypes = readFileSync(
      resolve(import.meta.dirname, '../package/business.d.ts'),
      'utf8',
    );
    const optionsBlock = businessTypes.match(
      /export interface BusinessAdminOptions \{([\s\S]*?)\n\}/,
    )?.[1];
    expect(optionsBlock).toContain('businessRoutes?: AdminBusinessRoute[];');
    expect(optionsBlock).not.toMatch(/^\s*routes\?:/m);
  });

  it('keeps the published CLI available to a clean Git checkout', () => {
    const cli = resolve(import.meta.dirname, '../bin/mss-admin-web.cjs');
    const biomeConfig = resolve(import.meta.dirname, '../biome.json');
    const consumerSetup = resolve(import.meta.dirname, './setup.ts');
    expect(existsSync(cli)).toBe(true);
    expect(existsSync(biomeConfig)).toBe(true);
    expect(packageManifest.files).toContain('biome.json');
    expect(readFileSync(cli, 'utf8')).toContain(
      "['check', '--config-path', resolve(packageRoot, 'biome.json'), '.']",
    );
    const vitestConfig = readFileSync(
      resolve(import.meta.dirname, '../package/vitest.config.mjs'),
      'utf8',
    );
    expect(vitestConfig).toContain("import { existsSync } from 'node:fs'");
    expect(vitestConfig).toContain('src/route-registrations.ts');
    expect(vitestConfig).toContain('src/generated/routes.ts');
    expect(vitestConfig).toContain('existsSync(managedRouteRegistrations)');
    expect(readFileSync(consumerSetup, 'utf8')).not.toContain("import '@testing-library/dom'");
    const biome = JSON.parse(readFileSync(biomeConfig, 'utf8')) as {
      overrides?: Array<{ includes?: string[] }>;
    };
    expect(biome.overrides?.[0]?.includes).toContain('**/config/business-routes.generated.ts');
    expect(() =>
      execFileSync('git', ['check-ignore', '--quiet', cli], {
        cwd: resolve(import.meta.dirname, '..'),
        stdio: 'ignore',
      }),
    ).toThrow();
  });

  it('places business routes before the fail-closed fallbacks', () => {
    const routes = createAdminRoutes({
      businessRoutes: [{ path: '/contracts', component: '@/business/contracts' }],
    });
    const paths = routes.map((route) => route.path);
    expect(paths.indexOf('/contracts')).toBeGreaterThan(paths.indexOf('/workplace'));
    expect(paths.indexOf('/contracts')).toBeLessThan(paths.indexOf('/403'));
    expect(paths.slice(-3)).toEqual(['/403', '/', '/*']);
    expect(
      routes
        .flatMap((route) => [route, ...(route.routes ?? [])])
        .filter((route) => route.path !== '/contracts' && route.component)
        .every((route) => isAbsolute(route.component ?? '')),
    ).toBe(true);
  });

  it('keeps complete Admin routes immutable at the public configuration boundary', () => {
    expect(() => defineBusinessAdmin({ routes: [] })).toThrow('Admin core routes are immutable');
    expect(() => defineBusinessAdmin({ businessRoutes: {} })).toThrow(
      'Admin Web businessRoutes must be an array',
    );

    const inheritedOverride = Object.create({ routes: [] }) as Record<string, unknown>;
    expect(() => defineBusinessAdmin(inheritedOverride)).toThrow('Admin core routes are immutable');

    const configuration = defineBusinessAdmin({
      businessRoutes: [{ path: '/contracts', component: '@/business/contracts' }],
    }) as { routes: Route[] };
    const paths = configuration.routes.map((route) => route.path);
    expect(paths).toContain('/workplace');
    expect(paths).toContain('/contracts');
    expect(paths.slice(-3)).toEqual(['/403', '/', '/*']);
  });

  it('keeps the retired MFSU runtime out of external development builds', () => {
    const tailwindPostCSS = require.resolve('@tailwindcss/postcss');
    expect(defineBusinessAdmin()).toMatchObject({ mfsu: false, utoopack: false });
    expect(defineBusinessAdmin({ useUtoopack: true })).toMatchObject({
      alias: { tailwindcss: expect.stringContaining('tailwindcss') },
      mfsu: false,
      utoopack: {
        resolve: {
          alias: { tailwindcss: expect.stringContaining('tailwindcss') },
        },
        styles: {
          postcss: {
            plugins: {
              [tailwindPostCSS]: {},
            },
          },
        },
      },
    });
  });

  it('deduplicates release dependencies across initial and lazy route chunks', () => {
    const businessPath = resolve(import.meta.dirname, '../package/business.cjs');
    const output = execFileSync(
      process.execPath,
      [
        '-e',
        `const { defineBusinessAdmin } = require(${JSON.stringify(businessPath)});
const path = require('node:path');
const packageRoot = path.resolve(path.dirname(${JSON.stringify(businessPath)}), '..');
let groups;
const config = defineBusinessAdmin();
const memo = {
  plugin: () => ({ use: () => undefined }),
  optimization: {
    runtimeChunk: () => undefined,
    splitChunks: (value) => { groups = value.cacheGroups; },
  },
};
config.chainWebpack(memo);
const moduleAt = (resource) => ({ nameForCondition: () => resource });
console.log(JSON.stringify({
  chunks: Object.fromEntries(Object.entries(groups).map(([name, value]) => [name, value.chunks])),
  vendorMatches: {
    packageSource: groups.vendors.test(moduleAt(path.resolve(packageRoot, 'src/node_modules/fixture/page.tsx'))),
    packageDependency: groups.vendors.test(moduleAt(path.resolve(packageRoot, 'node_modules/react/index.js'))),
    externalDependency: groups.vendors.test(moduleAt(path.resolve(packageRoot, '../host/node_modules/dayjs/index.js'))),
    hostSource: groups.vendors.test(moduleAt(path.resolve(packageRoot, '../host/src/business/page.tsx'))),
  },
}));`,
      ],
      {
        cwd: resolve(import.meta.dirname, '..'),
        encoding: 'utf8',
        env: { ...process.env, UMI_ENV: 'release' },
      },
    );
    expect(JSON.parse(output)).toEqual({
      chunks: {
        antDesignRuntime: 'all',
        antdRuntime: 'all',
        applicationShell: 'all',
        proComponents: 'all',
        queryRuntime: 'all',
        rcRuntime: 'all',
        umiRuntime: 'all',
        vendors: 'all',
      },
      vendorMatches: {
        packageSource: false,
        packageDependency: true,
        externalDependency: true,
        hostSource: false,
      },
    });
  });

  it('resolves generated Tailwind CSS from the Admin Web dependency tree', () => {
    const absTmpPath = resolve(import.meta.dirname, 'fixtures/external-host/.umi');
    let generateFiles: (() => void) | undefined;
    let generatedFile: { content: string; path: string } | undefined;
    type DevMiddleware = (
      request: { path?: string },
      response: {
        sendFile: (path: string) => void;
        setHeader: (name: string, value: string) => void;
      },
      next: () => void,
    ) => void;
    const middlewareFactories: Array<() => DevMiddleware[]> = [];
    const previousNonce = process.env.MSS_DEV_HEALTH_NONCE;
    process.env.MSS_DEV_HEALTH_NONCE = 'launch-nonce-for-test';
    mssAdminPlugin({
      EnableBy: { config: 'config' },
      cwd: resolve(import.meta.dirname, 'fixtures/external-host'),
      paths: {
        absOutputPath: resolve(import.meta.dirname, 'fixtures/external-host/dist'),
        absSrcPath: resolve(import.meta.dirname, 'fixtures/external-host/src'),
        absTmpPath,
      },
      config: { mssAdmin: {} },
      describe: () => undefined,
      modifyConfig: () => undefined,
      modifyTSConfig: () => undefined,
      onGenerateFiles: (callback: () => void) => {
        generateFiles = callback;
      },
      writeTmpFile: (file: { content: string; path: string }) => {
        generatedFile = file;
      },
      addEntryImportsAhead: () => undefined,
      addBeforeMiddlewares: (factory: () => DevMiddleware[]) => {
        middlewareFactories.push(factory);
      },
      onBuildComplete: () => undefined,
      addTmpGenerateWatcherPaths: () => undefined,
    });
    const headers = new Map<string, string>();
    for (const factory of middlewareFactories) {
      for (const middleware of factory()) {
        middleware(
          { path: '/' },
          {
            sendFile: () => undefined,
            setHeader: (name: string, value: string) => headers.set(name, value),
          },
          () => undefined,
        );
      }
    }
    expect(headers.get('X-MSS-Dev-Launch')).toBe('launch-nonce-for-test');
    if (previousNonce === undefined) delete process.env.MSS_DEV_HEALTH_NONCE;
    else process.env.MSS_DEV_HEALTH_NONCE = previousNonce;

    expect(generateFiles).toBeTypeOf('function');
    generateFiles?.();
    expect(generatedFile).toBeDefined();
    const generated = generatedFile as { content: string; path: string };
    const importMatch = generated.content.match(/^@import "([^"]+)";/m);
    expect(importMatch).not.toBeNull();
    expect(resolve(absTmpPath, importMatch?.[1] ?? '')).toBe(
      require.resolve('tailwindcss/index.css'),
    );
    expect(generated.content).not.toContain('@import "tailwindcss";');
  });

  it('creates the stats output directory before emitting the runtime graph', () => {
    const temporaryRoot = mkdtempSync(resolve(tmpdir(), 'mss-admin-runtime-stats-'));
    const outputPath = resolve(temporaryRoot, 'nested/dist');
    let done: ((stats: { toJson: () => { modules: Array<{ name: string }> } }) => void) | undefined;
    try {
      new RuntimeStatsPlugin().apply({
        outputPath,
        hooks: {
          done: {
            tap: (
              _name: string,
              callback: (stats: { toJson: () => { modules: Array<{ name: string }> } }) => void,
            ) => {
              done = callback;
            },
          },
        },
      });
      expect(done).toBeTypeOf('function');
      done?.({ toJson: () => ({ modules: [{ name: 'external-business-module' }] }) });
      expect(JSON.parse(readFileSync(resolve(outputPath, 'stats.json'), 'utf8'))).toEqual({
        modules: [{ name: 'external-business-module' }],
      });
    } finally {
      rmSync(temporaryRoot, { force: true, recursive: true });
    }
  });

  it('rejects reserved and duplicate business routes', () => {
    expect(() => createAdminRoutes({ businessRoutes: [{ path: '/menu' }] })).toThrow(
      'business route conflicts',
    );
    expect(() => createAdminRoutes({ businessRoutes: [{ path: '/role/audit' }] })).toThrow(
      '/role/:id',
    );
    expect(() => createAdminRoutes({ businessRoutes: [{ path: '/user/custom-login' }] })).toThrow(
      '/user/*',
    );
    expect(() =>
      createAdminRoutes({ businessRoutes: [{ path: '/contracts' }, { path: '/contracts' }] }),
    ).toThrow('duplicate business route');
    expect(() =>
      createAdminRoutes({
        businessRoutes: [{ path: '/contracts/:id' }, { path: '/contracts/create' }],
      }),
    ).toThrow('business route overlaps another business route');
  });
});
