import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { cleanup, render, screen } from '@testing-library/react';
import type { PropsWithChildren, ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import AdministrationPage from './Administration';

const runtime = vi.hoisted(() => ({
  initialState: undefined as InitialState | undefined,
  pathname: '/users',
  presentationTitle: 'Users',
}));

function permissionFlags(props: Record<string, unknown>): string {
  return Object.values(props)
    .filter((value): value is boolean => typeof value === 'boolean')
    .join(':');
}

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id, locale: 'en-US' }),
  useLocation: () => ({ pathname: runtime.pathname }),
  useModel: () => ({ initialState: runtime.initialState }),
}));

vi.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children, title }: PropsWithChildren<{ title?: ReactNode }>) => (
    <main>
      {title}
      {children}
    </main>
  ),
}));

vi.mock('@mss-admin-core/shared/presentation/runtime', () => ({
  usePagePresentation: () => ({ model: { title: runtime.presentationTitle } }),
}));

vi.mock('@mss-admin-core/modules/administration/UserManagement', () => ({
  default: (props: Record<string, unknown>) => <div>{`users:${permissionFlags(props)}`}</div>,
}));
vi.mock('@mss-admin-core/modules/administration/RoleManagement', () => ({
  default: (props: Record<string, unknown>) => <div>{`roles:${permissionFlags(props)}`}</div>,
}));
vi.mock('@mss-admin-core/modules/administration/MenuManagement', () => ({
  default: (props: Record<string, unknown>) => <div>{`menus:${permissionFlags(props)}`}</div>,
}));
vi.mock('@mss-admin-core/modules/administration/DepartmentManagement', () => ({
  default: (props: Record<string, unknown>) => <div>{`departments:${permissionFlags(props)}`}</div>,
}));
vi.mock('@mss-admin-core/modules/administration/PostManagement', () => ({
  default: (props: Record<string, unknown>) => <div>{`posts:${permissionFlags(props)}`}</div>,
}));

function state(permissions: Record<string, boolean>, root = false): InitialState {
  return {
    currentUser: { id: 'user-1', role: { root }, permissions },
    settings: {},
    authorizedMenu: [],
    fetchCurrentUser: async () => undefined,
  };
}

function renderRoute(pathname: string) {
  runtime.pathname = pathname;
  return render(<AdministrationPage />);
}

describe('administration route guards', () => {
  beforeEach(() => {
    runtime.initialState = undefined;
    runtime.pathname = '/users';
    runtime.presentationTitle = 'Users';
  });
  afterEach(cleanup);

  it('fails closed before identity bootstrap and without exact read permission', () => {
    renderRoute('/users');
    expect(screen.getByText('403')).toBeTruthy();
    cleanup();

    runtime.initialState = state({ '/role': true });
    renderRoute('/users');
    expect(screen.getByText('403')).toBeTruthy();
  });

  it('allows delegated reads without exposing authority mutations', () => {
    runtime.initialState = state({
      '/users': true,
      '/role': true,
      '/menu': true,
      '/departments': true,
      '/posts': true,
    });

    renderRoute('/users');
    expect(screen.getByText('users:false:false:false:false')).toBeTruthy();
    cleanup();
    renderRoute('/role');
    expect(screen.getByText('roles:false:false:false:false')).toBeTruthy();
    cleanup();
    renderRoute('/menu');
    expect(screen.getByText('menus:false:false:false:false')).toBeTruthy();
    cleanup();
    renderRoute('/departments');
    expect(screen.getByText('departments:false:false:false')).toBeTruthy();
    cleanup();
    renderRoute('/posts');
    expect(screen.getByText('posts:false:false:false')).toBeTruthy();
  });

  it('treats a verified root identity as the only authority mutation principal', () => {
    runtime.initialState = state({}, true);

    renderRoute('/users');
    expect(screen.getByText('users:true:true:true:true')).toBeTruthy();
    cleanup();
    renderRoute('/role');
    expect(screen.getByText('roles:true:true:true:true')).toBeTruthy();
    cleanup();
    renderRoute('/menu');
    expect(screen.getByText('menus:true:true:true:true')).toBeTruthy();
  });

  it('uses the effective user presentation title without changing root mutation flags', () => {
    runtime.initialState = state({ '/users': true });
    runtime.presentationTitle = 'Directory operators';

    renderRoute('/users');

    expect(screen.getByRole('heading', { name: 'Directory operators' })).toBeTruthy();
    expect(screen.getByText('users:false:false:false:false')).toBeTruthy();
  });
});
