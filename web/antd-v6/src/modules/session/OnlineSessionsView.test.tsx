import {
  ADMIN_PRESENTATION_API_VERSION,
  ADMIN_PRESENTATION_KIND,
} from '@mss-admin-core/shared/presentation/contract';
import { resolveEffectivePagePresentation } from '@mss-admin-core/shared/presentation/runtime';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { cloneElement, isValidElement, type ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { OnlineSession, OnlineSessionPage } from './contract';
import OnlineSessionsView from './OnlineSessionsView';
import { onlineSessionPresentationRegistryEntry } from './tablePresentation';

const sessionQuery = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
const api = vi.hoisted(() => ({
  revokeOne: vi.fn(),
  revokeUser: vi.fn(),
}));
const invalidateQueries = vi.hoisted(() => vi.fn());
const tableRuntime = vi.hoisted(() => ({
  props: undefined as Record<string, unknown> | undefined,
}));

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
    Table: (props: {
      columns: Array<{
        dataIndex?: string;
        key?: string;
        title?: ReactNode;
        width?: number;
        render?: (value: unknown, row: OnlineSession) => ReactNode;
      }>;
      dataSource: OnlineSession[];
      locale?: { emptyText?: ReactNode };
    }) => {
      tableRuntime.props = props;
      const { columns, dataSource, locale } = props;
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

vi.mock('@mss-admin-core/shared/query/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@mss-admin-core/shared/query/client')>();
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

const rootUser = { id: 'root-user', role: { root: true }, permissions: {} };

function compiledRuntime() {
  return resolveEffectivePagePresentation({
    entry: onlineSessionPresentationRegistryEntry,
    locale: 'en-US',
    user: rootUser,
    settled: true,
  });
}

function activeRuntime() {
  const definitionHash = onlineSessionPresentationRegistryEntry.definitionHash;
  return resolveEffectivePagePresentation({
    entry: onlineSessionPresentationRegistryEntry,
    locale: 'en-US',
    user: rootUser,
    settled: true,
    response: {
      pageKey: 'online-session.list',
      definitionHash,
      adoption: {
        mode: 'active',
        state: 'active',
        resolveLayers: true,
        applyLayers: true,
      },
      diagnostics: [],
      layers: {
        application: {
          apiVersion: ADMIN_PRESENTATION_API_VERSION,
          kind: ADMIN_PRESENTATION_KIND,
          metadata: {
            name: 'online-session-list-active-test',
            pageKey: 'online-session.list',
            definitionHash,
            scope: { kind: 'application' },
          },
          spec: {
            list: {
              columns: [
                { field: 'userAgent', hidden: true },
                {
                  field: 'status',
                  label: { 'en-US': 'Session state' },
                  order: 5,
                  width: 200,
                },
              ],
              density: 'compact',
              pageSize: 50,
            },
            search: {
              collapsedByDefault: true,
              fields: [
                { field: 'username', label: { 'en-US': 'Session user' } },
                { field: 'ip', hidden: true },
                { field: 'status', hidden: true },
              ],
            },
          },
        },
      },
    },
  });
}

function renderView(presentationRuntime = compiledRuntime()) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <App>
      <QueryClientProvider client={client}>
        <OnlineSessionsView presentationRuntime={presentationRuntime} />
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
    tableRuntime.props = undefined;
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
          <OnlineSessionsView presentationRuntime={compiledRuntime()} />
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
          <OnlineSessionsView presentationRuntime={compiledRuntime()} />
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

  it('applies active list and search presentation without changing protected revoke actions', () => {
    sessionQuery.current = { ...sessionQuery.current, data: page() };
    renderView(activeRuntime());

    const columns = tableRuntime.props?.columns as Array<Record<string, unknown>>;
    expect(columns.map((column) => column.key ?? column.dataIndex)).toEqual([
      'status',
      'username',
      'ip',
      'lastSeenAt',
      'actions',
    ]);
    expect(columns[0]).toMatchObject({ title: 'Session state', width: 200 });
    expect(tableRuntime.props).toMatchObject({ size: 'small' });
    expect(tableRuntime.props?.pagination).toMatchObject({ pageSize: 50 });

    expect(screen.queryByText('Session user')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /actions\.search/ }));
    expect(screen.getByText('Session user')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'sessions.action.revoke' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'sessions.action.revokeUser' })).toBeTruthy();
  });
});
