import {
  ADMIN_PRESENTATION_API_VERSION,
  ADMIN_PRESENTATION_KIND,
} from '@mss-admin-core/shared/presentation/contract';
import {
  type PresentationRegistryEntry,
  resolveEffectivePagePresentation,
} from '@mss-admin-core/shared/presentation/runtime';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { App } from 'antd';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import NoticeCenter from './NoticeCenter';
import SystemConfigManagement from './SystemConfigManagement';
import TaskManagement from './TaskManagement';
import {
  noticePresentationRegistryEntry,
  systemConfigPresentationRegistryEntry,
  taskPresentationRegistryEntry,
} from './tablePresentation';

const tableRuntime = vi.hoisted(() => ({
  props: undefined as Record<string, unknown> | undefined,
}));
const queryRuntime = vi.hoisted(() => ({
  notice: { current: {} as Record<string, unknown> },
  systemConfig: { current: {} as Record<string, unknown> },
  task: { current: {} as Record<string, unknown> },
}));

vi.mock('@mss-admin-core/shared/design-system/ResponsiveEntityTable', () => ({
  default: (props: Record<string, unknown>) => {
    tableRuntime.props = props;
    return <section data-testid="resolved-management-table" />;
  },
}));

vi.mock('@mss-admin-core/shared/navigation/managementRoute', () => ({
  finishManagementRouteIntent: vi.fn(),
  useManagementRouteIntent: vi.fn(),
}));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'en-US',
    formatMessage: ({ id }: { id: string }) => id,
  }),
  useSearchParams: () => [new URLSearchParams()],
}));

vi.mock('./api', () => ({
  operationsAPI: {
    notices: { markRead: vi.fn() },
    systemConfigs: { get: vi.fn() },
    tasks: { get: vi.fn() },
  },
}));

vi.mock('./query', () => ({
  useNotice: () => ({ data: undefined, error: null, isError: false, isPending: false }),
  useNoticePage: () => queryRuntime.notice.current,
  useSystemConfig: () => ({ data: undefined, error: null, isError: false, isPending: false }),
  useSystemConfigPage: () => queryRuntime.systemConfig.current,
  useTask: () => ({ data: undefined, error: null, isError: false, isPending: false }),
  useTaskFunctions: () => ({ data: [], error: null, isError: false, isPending: false }),
  useTaskPage: () => queryRuntime.task.current,
}));

const rootUser = {
  id: 'root-user',
  role: { root: true },
  permissions: {},
};

function activeRuntime(
  entry: PresentationRegistryEntry,
  columns: Array<{
    field: string;
    hidden?: boolean;
    label?: { 'en-US': string };
    order?: number;
  }>,
  searchFields?: Array<{
    field: string;
    hidden?: boolean;
    label?: { 'en-US': string };
  }>,
) {
  const definitionHash = entry.definitionHash;
  const pageKey = entry.definition.pageKey;
  return resolveEffectivePagePresentation({
    entry,
    locale: 'en-US',
    user: rootUser,
    settled: true,
    response: {
      pageKey,
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
            name: `${pageKey}-active-test`,
            pageKey,
            definitionHash,
            scope: { kind: 'application' },
          },
          spec: {
            list: { columns, density: 'compact', pageSize: 50 },
            ...(searchFields ? { search: { collapsedByDefault: true, fields: searchFields } } : {}),
          },
        },
      },
    },
  });
}

function page() {
  return { current: 1, data: [], pageSize: 20, total: 0 };
}

function query() {
  return {
    data: page(),
    error: null,
    isError: false,
    isFetching: false,
    isPending: false,
    refetch: vi.fn(),
  };
}

function renderView(view: React.ReactNode) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  return render(
    <App>
      <QueryClientProvider client={client}>{view}</QueryClientProvider>
    </App>,
  );
}

function columnKey(column: Record<string, unknown>): string {
  return String(column.key ?? column.dataIndex ?? '');
}

function resolvedTable() {
  return tableRuntime.props as {
    columns: Record<string, unknown>[];
    pagination: { pageSize: number };
    size: string;
  };
}

describe('operations management presentation consumption', () => {
  beforeEach(() => {
    queryRuntime.notice.current = query();
    queryRuntime.systemConfig.current = query();
    queryRuntime.task.current = query();
    tableRuntime.props = undefined;
  });

  afterEach(() => {
    cleanup();
    tableRuntime.props = undefined;
  });

  it('applies task.list in the real task component without adding root actions', () => {
    renderView(
      <TaskManagement
        presentationRuntime={activeRuntime(
          taskPresentationRegistryEntry,
          [
            { field: 'remark', hidden: true },
            { field: 'status', label: { 'en-US': 'Configured task state' }, order: 5 },
          ],
          [
            { field: 'name', label: { 'en-US': 'Configured task query' } },
            { field: 'status', hidden: true },
          ],
        )}
        root={false}
      />,
    );

    const table = resolvedTable();
    expect(table.columns[0]).toMatchObject({ dataIndex: 'status', title: 'Configured task state' });
    expect(table.columns.map(columnKey)).not.toContain('remark');
    expect(table.columns.map(columnKey)).not.toContain('actions');
    expect(table).toMatchObject({ pagination: { pageSize: 50 }, size: 'small' });
    expect(screen.queryByText('Configured task query')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /actions\.search/ }));
    expect(screen.getByText('Configured task query')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /task\.create\.action/ })).toBeNull();
  });

  it('applies notice.list in the real notice component without adding mark-read authority', () => {
    renderView(
      <NoticeCenter
        canMarkRead={false}
        presentationRuntime={activeRuntime(
          noticePresentationRegistryEntry,
          [
            { field: 'description', hidden: true },
            { field: 'status', label: { 'en-US': 'Configured notice state' }, order: 5 },
          ],
          [
            { field: 'title', label: { 'en-US': 'Configured notice query' } },
            { field: 'status', hidden: true },
          ],
        )}
      />,
    );

    const table = resolvedTable();
    expect(table.columns[0]).toMatchObject({
      dataIndex: 'status',
      title: 'Configured notice state',
    });
    expect(table.columns.map(columnKey)).not.toContain('description');
    expect(table.columns.map(columnKey)).toContain('actions');
    expect(table).toMatchObject({ pagination: { pageSize: 50 }, size: 'small' });
    expect(screen.queryByText('Configured notice query')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /actions\.search/ }));
    expect(screen.getByText('Configured notice query')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /notice\.read\.all/ })).toBeNull();
  });

  it('applies system-config.list in the real configuration component while retaining compiled actions', () => {
    renderView(
      <SystemConfigManagement
        presentationRuntime={activeRuntime(systemConfigPresentationRegistryEntry, [
          { field: 'remark', hidden: true },
          {
            field: 'updatedAt',
            label: { 'en-US': 'Configured update time' },
            order: 5,
          },
        ])}
      />,
    );

    const table = resolvedTable();
    expect(table.columns[0]).toMatchObject({
      dataIndex: 'updatedAt',
      title: 'Configured update time',
    });
    expect(table.columns.map(columnKey)).not.toContain('remark');
    expect(table.columns.map(columnKey)).toContain('actions');
    expect(table).toMatchObject({ pagination: { pageSize: 50 }, size: 'small' });
    expect(screen.getByRole('button', { name: /systemConfig\.create\.action/ })).toBeTruthy();
  });
});
