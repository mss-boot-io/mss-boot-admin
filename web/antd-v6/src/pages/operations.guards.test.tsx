import { cleanup, render, screen } from '@testing-library/react';
import type { PropsWithChildren } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { InitialState } from '@/shared/auth/types';
import OperationsPage from './Operations';

const runtime = vi.hoisted(() => ({
  initialState: undefined as InitialState | undefined,
  pathname: '/task',
}));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
  useLocation: () => ({ pathname: runtime.pathname }),
  useModel: () => ({ initialState: runtime.initialState }),
}));

vi.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: PropsWithChildren) => <main>{children}</main>,
}));
vi.mock('@/modules/operations/TaskManagement', () => ({
  default: ({ root }: { root: boolean }) => <div>{`task:${root}`}</div>,
}));
vi.mock('@/modules/operations/NoticeCenter', () => ({
  default: ({ canMarkRead }: { canMarkRead: boolean }) => <div>{`notice:${canMarkRead}`}</div>,
}));
vi.mock('@/modules/operations/LogViewer', () => ({
  default: ({ canExportRuntime, canReadRuntime }: Record<string, boolean>) => (
    <div>{`log:${canReadRuntime}:${canExportRuntime}`}</div>
  ),
}));
vi.mock('@/modules/operations/SystemConfigManagement', () => ({
  default: () => <div>system-config</div>,
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
  return render(<OperationsPage />);
}

describe('operations route guards', () => {
  beforeEach(() => {
    runtime.initialState = undefined;
    runtime.pathname = '/task';
  });
  afterEach(cleanup);

  it('fails closed before identity bootstrap and without exact read permission', () => {
    renderRoute('/task');
    expect(screen.getByText('403')).toBeTruthy();
    cleanup();

    runtime.initialState = state({ '/task/operate': true });
    renderRoute('/task');
    expect(screen.getByText('403')).toBeTruthy();
  });

  it('keeps task writes root-only and notice actions separately permissioned', () => {
    runtime.initialState = state({ '/task': true, '/notice': true });
    renderRoute('/task');
    expect(screen.getByText('task:false')).toBeTruthy();
    cleanup();

    renderRoute('/notice');
    expect(screen.getByText('notice:false')).toBeTruthy();
    cleanup();

    runtime.initialState = state({ '/notice': true, '/notice/read': true });
    renderRoute('/notice');
    expect(screen.getByText('notice:true')).toBeTruthy();
  });

  it('gates runtime log read and export independently', () => {
    runtime.initialState = state({
      '/log': true,
      '/log/runtime': true,
      '/log/export': false,
    });
    renderRoute('/log');
    expect(screen.getByText('log:true:false')).toBeTruthy();
  });

  it('allows only root to enter opaque system configuration', () => {
    runtime.initialState = state({ '/system-config': true });
    renderRoute('/system-config');
    expect(screen.getByText('403')).toBeTruthy();
    cleanup();

    runtime.initialState = state({}, true);
    renderRoute('/system-config');
    expect(screen.getByText('system-config')).toBeTruthy();
  });
});
