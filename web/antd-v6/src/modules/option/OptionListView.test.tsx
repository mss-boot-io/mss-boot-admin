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
import type { OptionPage, OptionSummary } from './contract';
import OptionListView from './OptionListView';
import { optionPresentationRegistryEntry } from './tablePresentation';

const optionQuery = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
const api = vi.hoisted(() => ({ remove: vi.fn() }));
const tableRuntime = vi.hoisted(() => ({
  props: undefined as Record<string, unknown> | undefined,
}));

vi.mock('@umijs/max', () => ({
  history: { push: vi.fn() },
  useIntl: () => ({ locale: 'en-US', formatMessage: ({ id }: { id: string }) => id }),
}));

vi.mock('antd', async (importOriginal) => {
  const actual = await importOriginal<typeof import('antd')>();
  return {
    ...actual,
    Table: (props: {
      columns: Array<{
        dataIndex?: string;
        key?: string;
        title?: ReactNode;
        width?: number;
        render?: (value: unknown, row: OptionSummary) => ReactNode;
      }>;
      dataSource: OptionSummary[];
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

const rootUser = { id: 'root-user', role: { root: true }, permissions: {} };

function compiledRuntime() {
  return resolveEffectivePagePresentation({
    entry: optionPresentationRegistryEntry,
    locale: 'en-US',
    user: rootUser,
    settled: true,
  });
}

function activeRuntime() {
  const definitionHash = optionPresentationRegistryEntry.definitionHash;
  return resolveEffectivePagePresentation({
    entry: optionPresentationRegistryEntry,
    locale: 'en-US',
    user: rootUser,
    settled: true,
    response: {
      pageKey: 'option.list',
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
            name: 'option-list-active-test',
            pageKey: 'option.list',
            definitionHash,
            scope: { kind: 'application' },
          },
          spec: {
            list: {
              columns: [
                { field: 'displayName', hidden: true },
                {
                  field: 'status',
                  label: { 'en-US': 'Dictionary state' },
                  order: 5,
                  width: 210,
                },
              ],
              density: 'compact',
              pageSize: 50,
            },
            search: {
              collapsedByDefault: true,
              fields: [
                { field: 'name', label: { 'en-US': 'Dictionary query' } },
                { field: 'category', hidden: true },
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
  const invalidate = vi.spyOn(client, 'invalidateQueries').mockResolvedValue(undefined);
  const view = render(
    <App>
      <QueryClientProvider client={client}>
        <OptionListView
          canCreate={false}
          canDelete
          canEdit={false}
          presentationRuntime={presentationRuntime}
        />
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
    tableRuntime.props = undefined;
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
          <OptionListView
            canCreate={false}
            canDelete
            canEdit={false}
            presentationRuntime={compiledRuntime()}
          />
        </QueryClientProvider>
      </App>,
    );
    expect(screen.getByText('403')).toBeTruthy();

    optionQuery.current = { ...optionQuery.current, data: page([]), error: null, isError: false };
    view.rerender(
      <App>
        <QueryClientProvider client={new QueryClient()}>
          <OptionListView
            canCreate={false}
            canDelete
            canEdit={false}
            presentationRuntime={compiledRuntime()}
          />
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

  it('applies active list and search presentation without changing protected actions', () => {
    optionQuery.current = { ...optionQuery.current, data: page() };
    renderView(activeRuntime());

    const columns = tableRuntime.props?.columns as Array<Record<string, unknown>>;
    expect(columns.map((column) => column.key ?? column.dataIndex)).toEqual([
      'status',
      'name',
      'category',
      'version',
      'updatedAt',
      'actions',
    ]);
    expect(columns[0]).toMatchObject({ title: 'Dictionary state', width: 210 });
    expect(tableRuntime.props).toMatchObject({ size: 'small' });
    expect(tableRuntime.props?.pagination).toMatchObject({ pageSize: 50 });

    expect(screen.queryByText('Dictionary query')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /actions\.search/ }));
    expect(screen.getByText('Dictionary query')).toBeTruthy();
    expect(screen.getByRole('button', { name: /actions\.delete/ })).toBeTruthy();
  });
});
