import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { cloneElement, isValidElement, type ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { OptionPage, OptionSummary } from './contract';
import OptionListView from './OptionListView';

const optionQuery = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
const api = vi.hoisted(() => ({ remove: vi.fn() }));

vi.mock('@umijs/max', () => ({
  history: { push: vi.fn() },
  useIntl: () => ({ locale: 'en-US', formatMessage: ({ id }: { id: string }) => id }),
}));

vi.mock('antd', async (importOriginal) => {
  const actual = await importOriginal<typeof import('antd')>();
  return {
    ...actual,
    Table: ({
      columns,
      dataSource,
      locale,
    }: {
      columns: Array<{
        key?: string;
        render?: (value: unknown, row: OptionSummary) => ReactNode;
      }>;
      dataSource: OptionSummary[];
      locale?: { emptyText?: ReactNode };
    }) => {
      const actions = columns.find((column) => column.key === 'actions');
      if (dataSource.length === 0) return <div>{locale?.emptyText}</div>;
      return (
        <div>
          {dataSource.map((row) => (
            <div key={row.id}>
              <span>{row.name}</span>
              {actions?.render?.(undefined, row)}
            </div>
          ))}
        </div>
      );
    },
    Popconfirm: ({ children, onConfirm }: { children: ReactNode; onConfirm?: () => void }) => {
      if (!isValidElement<{ onClick?: () => void }>(children)) return children;
      return cloneElement(children, {
        onClick: () => {
          children.props.onClick?.();
          onConfirm?.();
        },
      });
    },
  };
});

vi.mock('./query', () => ({ useOptionPage: () => optionQuery.current }));
vi.mock('./api', () => ({ optionAPI: api }));
vi.mock('./OptionDetailDrawer', () => ({ default: () => null }));

function option(builtIn = false): OptionSummary {
  return {
    id: builtIn ? 'built-in' : 'option-1',
    category: 'system',
    displayName: 'Status',
    name: builtIn ? 'built-in-status' : 'status',
    remark: '',
    status: 'enabled',
    version: 3,
    builtIn,
    updatedAt: '2026-08-15T00:00:00Z',
  };
}

function page(data: OptionSummary[] = [option()]): OptionPage {
  return { data, total: data.length, current: 1, pageSize: 20 };
}

function renderView() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidate = vi.spyOn(client, 'invalidateQueries').mockResolvedValue(undefined);
  const view = render(
    <App>
      <QueryClientProvider client={client}>
        <OptionListView canCreate={false} canDelete canEdit={false} />
      </QueryClientProvider>
    </App>,
  );
  return { ...view, invalidate };
}

describe('option list view', () => {
  beforeEach(() => {
    optionQuery.current = {
      data: undefined,
      error: null,
      isError: false,
      isFetching: false,
      isPending: false,
      refetch: vi.fn(),
    };
    api.remove.mockReset();
    api.remove.mockResolvedValue(undefined);
  });

  it('renders loading, permission-denied, and empty states explicitly', () => {
    optionQuery.current = { ...optionQuery.current, isPending: true };
    const view = renderView();
    expect(screen.getByRole('status')).toBeTruthy();

    optionQuery.current = {
      ...optionQuery.current,
      data: page(),
      error: { response: { status: 403 } },
      isError: true,
      isPending: false,
    };
    view.rerender(
      <App>
        <QueryClientProvider client={new QueryClient()}>
          <OptionListView canCreate={false} canDelete canEdit={false} />
        </QueryClientProvider>
      </App>,
    );
    expect(screen.getByText('403')).toBeTruthy();

    optionQuery.current = { ...optionQuery.current, data: page([]), error: null, isError: false };
    view.rerender(
      <App>
        <QueryClientProvider client={new QueryClient()}>
          <OptionListView canCreate={false} canDelete canEdit={false} />
        </QueryClientProvider>
      </App>,
    );
    expect(screen.getByText('option.empty')).toBeTruthy();
  });

  it('deletes the row-bound revision and invalidates option data', async () => {
    optionQuery.current = { ...optionQuery.current, data: page() };
    const { invalidate } = renderView();
    fireEvent.click(screen.getByRole('button', { name: /actions\.delete/ }));
    await waitFor(() => expect(api.remove).toHaveBeenCalledWith(option()));
    await waitFor(() => expect(invalidate).toHaveBeenCalled());
  });

  it('never exposes delete for a built-in dictionary', () => {
    optionQuery.current = { ...optionQuery.current, data: page([option(true)]) };
    renderView();
    expect(screen.queryByRole('button', { name: /actions\.delete/ })).toBeNull();
    expect(screen.getByText('option.builtIn')).toBeTruthy();
  });
});
