import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
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
        './runtime/locales/en-US',
        './runtime/locales/zh-CN',
        './styles',
        './testing',
      ].sort(),
    );
    expect(Object.keys(packageManifest.exports)).not.toContain('./src/*');
    expect(packageManifest.name).toBe('@mss-boot-io/admin-web');
    expect(packageManifest.private).not.toBe(true);
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
    expect(packageManifest.dependencies.vite).toBe('8.2.1');
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

  it('keeps the retired MFSU runtime out of external development builds', () => {
    expect(defineBusinessAdmin()).toMatchObject({ mfsu: false, utoopack: false });
    expect(defineBusinessAdmin({ useUtoopack: true })).toMatchObject({
      alias: { tailwindcss: expect.stringContaining('tailwindcss') },
      mfsu: false,
      utoopack: {
        resolve: {
          alias: { tailwindcss: expect.stringContaining('tailwindcss') },
        },
      },
    });
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
