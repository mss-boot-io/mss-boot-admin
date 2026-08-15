import { describe, expect, it } from 'vitest';
import type { CurrentUser } from '../auth/types';
import { retainRegisteredMenu } from './registry';

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
});
