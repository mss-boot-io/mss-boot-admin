import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { cloneElement, isValidElement, type ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { OnlineSession, OnlineSessionPage } from './contract';
import OnlineSessionsView from './OnlineSessionsView';

const sessionQuery = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
const api = vi.hoisted(() => ({
  revokeOne: vi.fn(),
  revokeUser: vi.fn(),
}));
const invalidateQueries = vi.hoisted(() => vi.fn());

vi.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'en-US',
    formatMessage: ({ id }: { id: string }, values?: Record<string, unknown>) =>
      values ? `${id}:${Object.values(values).join('|')}` : id,
  }),
}));

vi.mock('antd', async (importOriginal) => {
  const actual = await importOriginal<typeof import('antd')>();
  return {
    ...actual,
    Grid: {
      ...actual.Grid,
      useBreakpoint: () => ({ md: true }),
    },
    Table: ({
      columns,
      dataSource,
      locale,
    }: {
      columns: Array<{
        key?: string;
        render?: (value: unknown, row: OnlineSession) => ReactNode;
      }>;
      dataSource: OnlineSession[];
      locale?: { emptyText?: ReactNode };
    }) => {
      const actions = columns.find((column) => column.key === 'actions');
      if (dataSource.length === 0) return <div>{locale?.emptyText}</div>;
      return (
        <div>
          {dataSource.map((row) => (
            <div key={row.id}>
              <span>{row.username}</span>
              {actions?.render?.(undefined, row)}
            </div>
          ))}
        </div>
      );
    },
    Popconfirm: ({
      children,
      onConfirm,
    }: {
      children: ReactNode;
      onConfirm?: () => void | Promise<void>;
    }) => {
      if (!isValidElement<{ onClick?: () => void }>(children)) return children;
      return cloneElement(children, {
        onClick: () => {
          children.props.onClick?.();
          void onConfirm?.();
        },
      });
    },
  };
});

vi.mock('./query', () => ({
  useOnlineSessionPage: () => sessionQuery.current,
}));

vi.mock('./SessionDetailDrawer', () => ({ default: () => null }));

vi.mock('./api', () => ({ sessionAPI: api }));

vi.mock('@/shared/query/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/shared/query/client')>();
  return {
    ...actual,
    queryClient: { invalidateQueries },
  };
});

function onlineSession(): OnlineSession {
  return {
    id: 'session-1',
    userID: 'user-1',
    username: 'alice',
    loginAt: '2026-08-15T00:00:00Z',
    lastSeenAt: '2026-08-15T01:00:00Z',
    expiredAt: '2099-08-15T02:00:00Z',
    revoked: false,
    current: false,
  };
}

function page(data: OnlineSession[] = [onlineSession()]): OnlineSessionPage {
  return { data, total: data.length, current: 1, pageSize: 20 };
}

function renderView() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <App>
      <QueryClientProvider client={client}>
        <OnlineSessionsView />
      </QueryClientProvider>
    </App>,
  );
}

describe('online sessions view', () => {
  beforeEach(() => {
    sessionQuery.current = {
      data: undefined,
      error: null,
      isError: false,
      isFetching: false,
      isPending: false,
      refetch: vi.fn(),
    };
    api.revokeOne.mockReset();
    api.revokeOne.mockResolvedValue(undefined);
    api.revokeUser.mockReset();
    api.revokeUser.mockResolvedValue({ affected: 2, userID: 'user-1' });
    invalidateQueries.mockReset();
    invalidateQueries.mockResolvedValue(undefined);
  });

  it('renders loading, permission-denied, and empty states explicitly', () => {
    sessionQuery.current = { ...sessionQuery.current, isPending: true };
    const view = renderView();
    expect(screen.getByRole('status')).toBeTruthy();

    sessionQuery.current = {
      ...sessionQuery.current,
      data: page(),
      error: { response: { status: 403 } },
      isError: true,
      isPending: false,
    };
    view.rerender(
      <App>
        <QueryClientProvider client={new QueryClient()}>
          <OnlineSessionsView />
        </QueryClientProvider>
      </App>,
    );
    expect(screen.getByText('403')).toBeTruthy();
    expect(screen.queryByText('alice')).toBeNull();

    sessionQuery.current = {
      ...sessionQuery.current,
      data: page([]),
      error: null,
      isError: false,
    };
    view.rerender(
      <App>
        <QueryClientProvider client={new QueryClient()}>
          <OnlineSessionsView />
        </QueryClientProvider>
      </App>,
    );
    expect(screen.getByText('sessions.empty')).toBeTruthy();
  });

  it('keeps the last successful page visible when a background refresh fails', () => {
    sessionQuery.current = {
      ...sessionQuery.current,
      data: page(),
      error: new Error('temporary'),
      isError: true,
    };

    renderView();

    expect(screen.getByText('sessions.refreshFailed')).toBeTruthy();
    expect(screen.getByText('alice')).toBeTruthy();
  });

  it('revokes only row-bound session and user identifiers, then invalidates session data', async () => {
    sessionQuery.current = { ...sessionQuery.current, data: page() };
    renderView();

    fireEvent.click(screen.getByRole('button', { name: 'sessions.action.revoke' }));
    await waitFor(() => expect(api.revokeOne).toHaveBeenCalledWith('session-1'));
    await waitFor(() => expect(invalidateQueries).toHaveBeenCalled());

    const revokeUser = screen.getByRole('button', { name: 'sessions.action.revokeUser' });
    await waitFor(() => expect((revokeUser as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(revokeUser);
    await waitFor(() => expect(api.revokeUser).toHaveBeenCalledWith('user-1'));
    await waitFor(() => expect(invalidateQueries).toHaveBeenCalledTimes(2));
  });

  it('fails closed when a revoke proves that authorization is no longer valid', async () => {
    api.revokeOne.mockRejectedValue({ response: { status: 403 } });
    sessionQuery.current = { ...sessionQuery.current, data: page() };
    renderView();

    fireEvent.click(screen.getByRole('button', { name: 'sessions.action.revoke' }));

    expect(await screen.findByText('403')).toBeTruthy();
    expect(screen.queryByText('alice')).toBeNull();
  });
});
