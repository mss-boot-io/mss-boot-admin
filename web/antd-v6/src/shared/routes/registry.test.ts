import { afterEach, describe, expect, it, vi } from 'vitest';
import type { CurrentUser } from '../auth/types';
import { retainRegisteredMenu, routeRegistry } from './registry';

const operator: CurrentUser = {
  id: 'operator',
  role: { root: false },
  permissions: { '/welcome': true },
};

describe('compiled route registry', () => {
  it('maps the backend welcome capability to the compiled v6 workplace route', () => {
    expect(
      retainRegisteredMenu(
        [{ id: 'welcome', name: 'welcome', path: '/welcome', component: './Welcome' }],
        operator,
      ),
    ).toEqual([
      {
        id: 'welcome',
        key: 'welcome',
        name: 'workplace',
        title: undefined,
        icon: undefined,
        type: undefined,
        hideChildrenInMenu: undefined,
        hideInMenu: undefined,
        hideInBreadcrumb: undefined,
        flatMenu: undefined,
        path: '/workplace',
        sourcePath: '/welcome',
        permission: '/welcome',
        rootOnly: undefined,
        children: undefined,
      },
    ]);
  });

  it('drops unknown executable paths while retaining a non-navigable directory', () => {
    const retained = retainRegisteredMenu(
      [
        {
          id: 'directory',
          path: '/unknown-parent',
          component: 'https://untrusted.example/module.js',
          children: [{ path: '/welcome' }, { path: '/database-component' }],
        },
      ],
      operator,
    );
    expect(retained).toHaveLength(1);
    expect(retained[0]).toMatchObject({
      key: 'directory',
      path: undefined,
      sourcePath: '/unknown-parent',
      children: [{ path: '/workplace', sourcePath: '/welcome' }],
    });
    expect(JSON.stringify(retained)).not.toContain('untrusted.example');
    expect(JSON.stringify(retained)).not.toContain('database-component');
  });

  it('fails closed when identity permissions disagree with the menu response', () => {
    expect(
      retainRegisteredMenu([{ path: '/welcome' }], {
        id: 'revoked',
        role: { root: false },
        permissions: { '/welcome': false },
      }),
    ).toEqual([]);
    expect(
      retainRegisteredMenu([{ path: '/welcome' }], {
        id: 'root',
        role: { root: true },
        permissions: {},
      }),
    ).toHaveLength(1);
  });

  it('registers application settings without trusting a server component string', () => {
    const menu = retainRegisteredMenu(
      [{ id: 'settings', path: '/app-config', component: 'UntrustedLegacyPage' }],
      {
        id: 'config-reader',
        role: { root: false },
        permissions: { '/app-config': true },
      },
    );
    expect(menu).toMatchObject([
      {
        id: 'settings',
        path: '/app-config',
        sourcePath: '/app-config',
        permission: '/app-config',
      },
    ]);
    expect(JSON.stringify(menu)).not.toContain('UntrustedLegacyPage');
  });

  it('registers the five compiled authority-management routes', () => {
    expect(
      ['/users', '/role', '/menu', '/departments', '/posts'].map((path) => routeRegistry.get(path)),
    ).toMatchObject([
      { permission: '/users', serverPaths: ['/users'] },
      { permission: '/role', serverPaths: ['/role'] },
      { permission: '/menu', serverPaths: ['/menu'] },
      { permission: '/departments', serverPaths: ['/departments'] },
      { permission: '/posts', serverPaths: ['/posts'] },
    ]);

    const retained = retainRegisteredMenu(
      [
        { path: '/users', component: 'UntrustedUserPage' },
        { path: '/role', component: 'https://untrusted.example/role.js' },
      ],
      {
        id: 'authority-reader',
        role: { root: false },
        permissions: { '/users': true, '/role': true },
      },
    );
    expect(retained.map((item) => item.path)).toEqual(['/users', '/role']);
    expect(JSON.stringify(retained)).not.toContain('UntrustedUserPage');
    expect(JSON.stringify(retained)).not.toContain('untrusted.example');
  });

  it('registers operational routes with exact component permissions and a root-only config boundary', () => {
    expect(
      ['/task', '/notice', '/log', '/system-config'].map((path) => routeRegistry.get(path)),
    ).toMatchObject([
      { permission: '/task', serverPaths: ['/task'] },
      { permission: '/notice', serverPaths: ['/notice'] },
      { permission: '/log', serverPaths: ['/log'] },
      { rootOnly: true, serverPaths: ['/system-config'] },
    ]);

    const delegated = retainRegisteredMenu(
      [{ path: '/task' }, { path: '/notice' }, { path: '/log' }, { path: '/system-config' }],
      {
        id: 'operator',
        role: { root: false },
        permissions: { '/task': true, '/notice': true, '/log': true, '/system-config': true },
      },
    );
    expect(delegated.map((item) => item.path)).toEqual(['/task', '/notice', '/log']);
  });

  it('loads generated supplier registration without executing backend component metadata', () => {
    expect(routeRegistry.get('/suppliers')).toMatchObject({
      menuName: 'supplier',
      permission: '/suppliers',
      serverPaths: ['/suppliers'],
    });

    const menu = retainRegisteredMenu(
      [{ id: 'suppliers', path: '/suppliers', component: 'https://untrusted.example/supplier.js' }],
      {
        id: 'supplier-reader',
        role: { root: false },
        permissions: { '/suppliers': true },
      },
    );
    expect(menu).toMatchObject([
      {
        id: 'suppliers',
        name: 'supplier',
        path: '/suppliers',
        permission: '/suppliers',
        sourcePath: '/suppliers',
      },
    ]);
    expect(JSON.stringify(menu)).not.toContain('untrusted.example');
  });

  it('retains the online-session route only for a root identity', () => {
    const serverMenu = [{ id: 'sessions', path: '/security/online-sessions' }];

    expect(retainRegisteredMenu(serverMenu, operator)).toEqual([]);
    expect(
      retainRegisteredMenu(serverMenu, {
        id: 'root',
        role: { root: true },
        permissions: {},
      }),
    ).toMatchObject([
      {
        id: 'sessions',
        name: 'online-sessions',
        path: '/security/online-sessions',
        rootOnly: true,
      },
    ]);
  });
});

describe('compiled route registry uniqueness', () => {
  afterEach(() => {
    vi.doUnmock('@mss-admin-business/routes');
    vi.resetModules();
  });

  it('fails closed when a business registration duplicates a core UI path', async () => {
    vi.resetModules();
    vi.doMock('@mss-admin-business/routes', () => ({
      default: [
        {
          path: '/workplace',
          serverPaths: ['/custom-workplace'],
          menuName: 'custom-workplace',
        },
      ],
    }));

    await expect(import('./registry')).rejects.toThrowError(
      '[mss-admin] duplicate UI route path "/workplace" between route registrations "workplace" (/workplace) and "custom-workplace" (/workplace).',
    );
  });

  it('fails closed when a business registration duplicates a core server path', async () => {
    vi.resetModules();
    vi.doMock('@mss-admin-business/routes', () => ({
      default: [
        {
          path: '/custom-welcome',
          serverPaths: ['/welcome'],
          menuName: 'custom-welcome',
        },
      ],
    }));

    await expect(import('./registry')).rejects.toThrowError(
      '[mss-admin] duplicate server route path "/welcome" between route registrations "workplace" (/workplace) and "custom-welcome" (/custom-welcome).',
    );
  });

  it('fails closed when business registrations duplicate a UI path', async () => {
    vi.resetModules();
    vi.doMock('@mss-admin-business/routes', () => ({
      default: [
        {
          path: '/business-duplicate',
          serverPaths: ['/business-one'],
          menuName: 'business-one',
        },
        {
          path: '/business-duplicate',
          serverPaths: ['/business-two'],
          menuName: 'business-two',
        },
      ],
    }));

    await expect(import('./registry')).rejects.toThrowError(
      '[mss-admin] duplicate UI route path "/business-duplicate" between route registrations "business-one" (/business-duplicate) and "business-two" (/business-duplicate).',
    );
  });

  it('fails closed when business registrations duplicate a server path', async () => {
    vi.resetModules();
    vi.doMock('@mss-admin-business/routes', () => ({
      default: [
        {
          path: '/business-one',
          serverPaths: ['/business-duplicate'],
          menuName: 'business-one',
        },
        {
          path: '/business-two',
          serverPaths: ['/business-duplicate'],
          menuName: 'business-two',
        },
      ],
    }));

    await expect(import('./registry')).rejects.toThrowError(
      '[mss-admin] duplicate server route path "/business-duplicate" between route registrations "business-one" (/business-one) and "business-two" (/business-two).',
    );
  });

  it('combines unique core and business registrations normally', async () => {
    vi.resetModules();
    vi.doMock('@mss-admin-business/routes', () => ({
      default: [
        {
          path: '/custom-health',
          serverPaths: ['/custom-health-api'],
          menuName: 'custom-health',
          permission: '/custom-health-api',
        },
      ],
    }));

    const registry = await import('./registry');
    expect(registry.routeRegistry.get('/workplace')).toMatchObject({
      menuName: 'workplace',
      serverPaths: ['/welcome'],
    });
    expect(registry.routeRegistry.get('/custom-health')).toMatchObject({
      menuName: 'custom-health',
      serverPaths: ['/custom-health-api'],
    });
    expect(
      registry.retainRegisteredMenu([{ id: 'custom-health', path: '/custom-health-api' }], {
        id: 'custom-reader',
        role: { root: false },
        permissions: { '/custom-health-api': true },
      }),
    ).toMatchObject([
      {
        id: 'custom-health',
        path: '/custom-health',
        sourcePath: '/custom-health-api',
        permission: '/custom-health-api',
      },
    ]);
  });
});
