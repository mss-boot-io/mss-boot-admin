import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { cloneElement, isValidElement, type ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { LanguagePage, LanguageSummary } from './contract';
import LanguageListView from './LanguageListView';

const languageQuery = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
const api = vi.hoisted(() => ({ remove: vi.fn() }));

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
    Table: ({
      columns,
      dataSource,
      locale,
    }: {
      columns: Array<{
        key?: string;
        render?: (value: unknown, row: LanguageSummary) => ReactNode;
      }>;
      dataSource: LanguageSummary[];
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

function renderView() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidate = vi.spyOn(client, 'invalidateQueries').mockResolvedValue(undefined);
  const view = render(
    <App>
      <QueryClientProvider client={client}>
        <LanguageListView canCreate={false} canDelete canEdit={false} />
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
          <LanguageListView canCreate={false} canDelete canEdit={false} />
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
          <LanguageListView canCreate={false} canDelete canEdit={false} />
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
});
