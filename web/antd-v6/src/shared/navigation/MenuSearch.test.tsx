import type { AuthorizedMenuItem } from '@mss-admin-core/shared/auth/types';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import MenuSearch, { buildAuthorizedMenuSearchItems } from './MenuSearch';

const { push } = vi.hoisted(() => ({ push: vi.fn() }));

const messages: Record<string, string> = {
  'menu.origination': '组织管理',
  'menu.origination.department': '部门管理',
  'menu.origination.user': '用户管理',
  'navigation.search.empty': '未找到匹配页面',
  'navigation.search.hint': '按 Enter 打开',
  'navigation.search.open': '打开菜单搜索',
  'navigation.search.placeholder': '搜索菜单',
};

vi.mock('@umijs/max', () => ({
  history: { push },
  useIntl: () => ({
    formatMessage: ({ defaultMessage, id }: { defaultMessage?: string; id: string }) =>
      messages[id] ?? defaultMessage ?? id,
  }),
}));

const menu: AuthorizedMenuItem[] = [
  {
    name: 'origination',
    path: '/origination',
    type: 'DIRECTORY',
    children: [
      { name: 'origination.user', path: '/users', type: 'MENU' },
      { name: 'origination.department', path: '/departments', type: 'MENU' },
      { name: 'hidden', path: '/hidden', type: 'MENU', hideInMenu: true },
      { name: 'detail', path: '/users/:id', type: 'MENU' },
    ],
  },
];

beforeEach(() => push.mockReset());

describe('authorized menu search', () => {
  it('indexes only visible, concrete routes and retains the localized hierarchy', () => {
    const items = buildAuthorizedMenuSearchItems(menu, (name) => messages[`menu.${name}`] ?? '');

    expect(items).toEqual([
      {
        label: '组织管理 / 用户管理',
        path: '/users',
        searchText: '组织管理 / 用户管理 origination.user /users',
      },
      {
        label: '组织管理 / 部门管理',
        path: '/departments',
        searchText: '组织管理 / 部门管理 origination.department /departments',
      },
    ]);
  });

  it('opens the first filtered result with Enter', async () => {
    render(
      <App>
        <MenuSearch items={menu} />
      </App>,
    );

    fireEvent.click(screen.getByRole('button', { name: '打开菜单搜索' }));
    const input = await screen.findByRole('combobox', { name: '搜索菜单' });
    fireEvent.change(input, { target: { value: '用户' } });
    await screen.findByText('组织管理 / 用户管理');
    fireEvent.keyDown(input, { code: 'Enter', key: 'Enter', keyCode: 13, which: 13 });

    await waitFor(() => expect(push).toHaveBeenCalledWith('/users'));
  });
});
