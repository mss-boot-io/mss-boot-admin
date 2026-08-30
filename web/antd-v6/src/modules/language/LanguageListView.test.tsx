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
import type { LanguagePage, LanguageSummary } from './contract';
import LanguageListView from './LanguageListView';
import { languagePresentationRegistryEntry } from './tablePresentation';

const languageQuery = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
const api = vi.hoisted(() => ({ remove: vi.fn() }));
const tableRuntime = vi.hoisted(() => ({
  props: undefined as Record<string, unknown> | undefined,
}));

vi.mock('@umijs/max', () => ({
  Link: ({ children, to }: { children: ReactNode; to: string }) => <a href={to}>{children}</a>,
  useIntl: () => ({
    locale: 'en-US',
    formatMessage: ({ id }: { id: string }) => id,
  }),
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
        render?: (value: unknown, row: LanguageSummary) => ReactNode;
      }>;
      dataSource: LanguageSummary[];
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

vi.mock('./query', () => ({ useLanguagePage: () => languageQuery.current }));
vi.mock('./api', () => ({ languageAPI: api }));
vi.mock('./LanguageDetailDrawer', () => ({ default: () => null }));

function language(): LanguageSummary {
  return {
    id: 'language-1',
    name: 'en-US',
    remark: '',
    status: 'enabled',
    updatedAt: '2026-08-15T00:00:00Z',
  };
}

function page(data: LanguageSummary[] = [language()]): LanguagePage {
  return { data, total: data.length, current: 1, pageSize: 20 };
}

const rootUser = { id: 'root-user', role: { root: true }, permissions: {} };

function compiledRuntime() {
  return resolveEffectivePagePresentation({
    entry: languagePresentationRegistryEntry,
    locale: 'en-US',
    user: rootUser,
    settled: true,
  });
}

function activeRuntime() {
  const definitionHash = languagePresentationRegistryEntry.definitionHash;
  return resolveEffectivePagePresentation({
    entry: languagePresentationRegistryEntry,
    locale: 'en-US',
    user: rootUser,
    settled: true,
    response: {
      pageKey: 'language.list',
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
            name: 'language-list-active-test',
            pageKey: 'language.list',
            definitionHash,
            scope: { kind: 'application' },
          },
          spec: {
            list: {
              columns: [
                { field: 'remark', hidden: true },
                {
                  field: 'status',
                  label: { 'en-US': 'Language state' },
                  order: 5,
                  width: 222,
                },
              ],
              density: 'compact',
              pageSize: 50,
            },
            search: {
              collapsedByDefault: true,
              fields: [
                { field: 'name', label: { 'en-US': 'Language query' } },
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
        <LanguageListView
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

describe('language list view', () => {
  beforeEach(() => {
    languageQuery.current = {
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
    languageQuery.current = { ...languageQuery.current, isPending: true };
    const view = renderView();
    expect(screen.getByRole('status')).toBeTruthy();

    languageQuery.current = {
      ...languageQuery.current,
      data: page(),
      error: { response: { status: 403 } },
      isError: true,
      isPending: false,
    };
    view.rerender(
      <App>
        <QueryClientProvider client={new QueryClient()}>
          <LanguageListView
            canCreate={false}
            canDelete
            canEdit={false}
            presentationRuntime={compiledRuntime()}
          />
        </QueryClientProvider>
      </App>,
    );
    expect(screen.getByText('403')).toBeTruthy();

    languageQuery.current = {
      ...languageQuery.current,
      data: page([]),
      error: null,
      isError: false,
    };
    view.rerender(
      <App>
        <QueryClientProvider client={new QueryClient()}>
          <LanguageListView
            canCreate={false}
            canDelete
            canEdit={false}
            presentationRuntime={compiledRuntime()}
          />
        </QueryClientProvider>
      </App>,
    );
    expect(screen.getByText('language.empty')).toBeTruthy();
  });

  it('deletes only the row-bound identifier and invalidates language data', async () => {
    languageQuery.current = { ...languageQuery.current, data: page() };
    const { invalidate } = renderView();

    fireEvent.click(screen.getByRole('button', { name: /actions\.delete/ }));

    await waitFor(() => expect(api.remove).toHaveBeenCalledWith('language-1'));
    await waitFor(() => expect(invalidate).toHaveBeenCalled());
  });

  it('applies active list and search presentation without changing protected actions', () => {
    languageQuery.current = { ...languageQuery.current, data: page() };
    renderView(activeRuntime());

    const columns = tableRuntime.props?.columns as Array<Record<string, unknown>>;
    expect(columns.map((column) => column.key ?? column.dataIndex)).toEqual([
      'status',
      'name',
      'updatedAt',
      'actions',
    ]);
    expect(columns[0]).toMatchObject({ title: 'Language state', width: 222 });
    expect(tableRuntime.props).toMatchObject({ size: 'small' });
    expect(tableRuntime.props?.pagination).toMatchObject({ pageSize: 50 });

    expect(screen.queryByText('Language query')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /actions\.search/ }));
    expect(screen.getByText('Language query')).toBeTruthy();
    expect(screen.queryByText('Status')).toBeNull();
    expect(screen.getByRole('button', { name: /actions\.delete/ })).toBeTruthy();
  });
});
