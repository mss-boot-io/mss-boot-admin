import {
  ADMIN_PRESENTATION_API_VERSION,
  ADMIN_PRESENTATION_KIND,
  type PagePresentationSpec,
} from '@mss-admin-core/shared/presentation/contract';
import {
  type PagePresentationRuntime,
  type PresentationRegistryEntry,
  resolveEffectivePagePresentation,
} from '@mss-admin-core/shared/presentation/runtime';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import LogViewer from './LogViewer';
import {
  auditLogPresentationRegistryEntry,
  loginLogPresentationRegistryEntry,
  runtimeLogPresentationRegistryEntry,
} from './tablePresentation';

const tableRuntime = vi.hoisted(() => ({
  propsByScrollWidth: new Map<number, Record<string, unknown>>(),
}));

vi.mock('@mss-admin-core/shared/design-system/ResponsiveEntityTable', () => ({
  default: (props: Record<string, unknown>) => {
    const width = Number((props.scroll as { x?: number } | undefined)?.x ?? 0);
    tableRuntime.propsByScrollWidth.set(width, props);
    return <section data-testid={`table-${width}`} />;
  },
}));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'en-US',
    formatMessage: ({ id }: { id: string }) => id,
  }),
}));

vi.mock('./query', () => {
  const readyQuery = () => ({
    error: null,
    isError: false,
    isFetching: false,
    isPending: false,
    refetch: vi.fn(),
  });
  return {
    useLoginLogPage: () => ({
      ...readyQuery(),
      data: { current: 1, data: [], pageSize: 20, total: 0 },
    }),
    useAuditLogPage: () => ({
      ...readyQuery(),
      data: { current: 1, data: [], pageSize: 20, total: 0 },
    }),
    useRuntimeLogPage: () => ({
      ...readyQuery(),
      data: { list: [], total: 0, truncated: false },
    }),
    useRuntimeLogFiles: () => ({
      ...readyQuery(),
      data: { files: [], truncated: false },
    }),
  };
});

vi.mock('./api', () => ({
  runtimeLogExportPath: () => '/compiled/runtime-log-export',
}));

const rootUser = {
  id: 'root-user',
  role: { root: true },
  permissions: {},
};

function compiledRuntime(entry: PresentationRegistryEntry): PagePresentationRuntime {
  return resolveEffectivePagePresentation({
    entry,
    locale: 'en-US',
    user: rootUser,
    settled: true,
  });
}

function activeRuntime(
  entry: PresentationRegistryEntry,
  name: string,
  spec: PagePresentationSpec,
): PagePresentationRuntime {
  return resolveEffectivePagePresentation({
    entry,
    locale: 'en-US',
    user: rootUser,
    settled: true,
    response: {
      pageKey: entry.definition.pageKey,
      definitionHash: entry.definitionHash,
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
            name,
            pageKey: entry.definition.pageKey,
            definitionHash: entry.definitionHash,
            scope: { kind: 'application' },
          },
          spec,
        },
      },
    },
  });
}

function columnKey(column: Record<string, unknown>): string {
  return String(column.key ?? column.dataIndex ?? '');
}

function tableProps(width: number) {
  return tableRuntime.propsByScrollWidth.get(width) as {
    columns: Record<string, unknown>[];
    pagination: { pageSize: number };
    size: string;
  };
}

function renderViewer(input?: {
  audit?: PagePresentationRuntime;
  canExportRuntime?: boolean;
  canReadRuntime?: boolean;
  login?: PagePresentationRuntime;
  runtime?: PagePresentationRuntime;
}) {
  return render(
    <App>
      <LogViewer
        auditPresentationRuntime={
          input?.audit ?? compiledRuntime(auditLogPresentationRegistryEntry)
        }
        canExportRuntime={input?.canExportRuntime ?? false}
        canReadRuntime={input?.canReadRuntime ?? true}
        loginPresentationRuntime={
          input?.login ?? compiledRuntime(loginLogPresentationRegistryEntry)
        }
        runtimePresentationRuntime={
          input?.runtime ?? compiledRuntime(runtimeLogPresentationRegistryEntry)
        }
      />
    </App>,
  );
}

describe('log viewer presentation consumption', () => {
  beforeEach(() => {
    tableRuntime.propsByScrollWidth.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('keeps compiled tabs, tables, and runtime permission gates by default', async () => {
    const login = compiledRuntime(loginLogPresentationRegistryEntry);
    const audit = compiledRuntime(auditLogPresentationRegistryEntry);
    const runtime = compiledRuntime(runtimeLogPresentationRegistryEntry);
    const view = renderViewer({ audit, canReadRuntime: false, login, runtime });

    expect(screen.getByRole('tab', { name: login.model.title })).toBeTruthy();
    expect(screen.getByRole('tab', { name: audit.model.title })).toBeTruthy();
    expect(screen.queryByRole('tab', { name: runtime.model.title })).toBeNull();
    expect(tableProps(920).columns.map(columnKey)).toEqual([
      'username',
      'ip',
      'location',
      'status',
      'message',
      'loginAt',
    ]);
    expect(tableProps(920)).toMatchObject({ pagination: { pageSize: 20 }, size: 'large' });

    view.rerender(
      <App>
        <LogViewer
          auditPresentationRuntime={audit}
          canExportRuntime={false}
          canReadRuntime
          loginPresentationRuntime={login}
          runtimePresentationRuntime={runtime}
        />
      </App>,
    );
    fireEvent.click(screen.getByRole('tab', { name: runtime.model.title }));
    await waitFor(() => expect(screen.getByTestId('table-760')).toBeTruthy());
    expect(screen.queryByRole('link', { name: /log\.export/ })).toBeNull();

    view.rerender(
      <App>
        <LogViewer
          auditPresentationRuntime={audit}
          canExportRuntime
          canReadRuntime
          loginPresentationRuntime={login}
          runtimePresentationRuntime={runtime}
        />
      </App>,
    );
    expect(screen.getByRole('link', { name: /log\.export/ }).getAttribute('href')).toBe(
      '/compiled/runtime-log-export',
    );
  });

  it('applies each active tab model without creating actions or remote renderers', async () => {
    const login = activeRuntime(loginLogPresentationRegistryEntry, 'active-login-log', {
      title: { 'en-US': 'Sign-in trail' },
      list: {
        columns: [
          { field: 'location', hidden: true },
          { field: 'status', label: { 'en-US': 'Outcome' }, order: 5, width: 222 },
        ],
        density: 'compact',
        pageSize: 50,
      },
      search: {
        collapsedByDefault: true,
        fields: [{ field: 'username', label: { 'en-US': 'Actor' } }],
      },
    });
    const audit = activeRuntime(auditLogPresentationRegistryEntry, 'active-audit-log', {
      title: { 'en-US': 'Audit trail' },
      list: {
        columns: [
          { field: 'message', hidden: true },
          { field: 'createdAt', label: { 'en-US': 'Recorded' }, order: 1, width: 240 },
        ],
        density: 'middle',
        pageSize: 100,
      },
      search: {
        collapsedByDefault: false,
        fields: [
          { field: 'username', label: { 'en-US': 'Principal' } },
          { field: 'type', hidden: true },
        ],
      },
    });
    const runtime = activeRuntime(runtimeLogPresentationRegistryEntry, 'active-runtime-log', {
      title: { 'en-US': 'Runtime stream' },
      list: {
        columns: [
          { field: 'message', hidden: true },
          { field: 'level', label: { 'en-US': 'Severity' }, order: 1, width: 160 },
        ],
        density: 'compact',
        pageSize: 50,
      },
      search: {
        collapsedByDefault: false,
        fields: [
          { field: 'keyword', label: { 'en-US': 'Contains' }, order: 1 },
          { field: 'level', label: { 'en-US': 'Minimum severity' }, order: 2 },
          { field: 'timeRange', hidden: true },
        ],
      },
    });

    renderViewer({ audit, canExportRuntime: true, login, runtime });
    expect(screen.getByRole('tab', { name: 'Sign-in trail' })).toBeTruthy();
    expect(screen.getByRole('tab', { name: 'Audit trail' })).toBeTruthy();
    expect(screen.getByRole('tab', { name: 'Runtime stream' })).toBeTruthy();

    expect(tableProps(920).columns.map(columnKey)).toEqual([
      'status',
      'username',
      'ip',
      'message',
      'loginAt',
    ]);
    expect(tableProps(920).columns[0]).toMatchObject({ title: 'Outcome', width: 222 });
    expect(tableProps(920)).toMatchObject({ pagination: { pageSize: 50 }, size: 'small' });
    expect(screen.queryByLabelText('Actor')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /actions\.search/ }));
    expect(await screen.findByLabelText('Actor')).toBeTruthy();

    fireEvent.click(screen.getByRole('tab', { name: 'Audit trail' }));
    await waitFor(() => expect(screen.getByTestId('table-1080')).toBeTruthy());
    expect(tableProps(1_080).columns.map(columnKey)).toEqual([
      'createdAt',
      'username',
      'type',
      'action',
      'resource',
      'path',
      'status',
      'duration',
    ]);
    expect(tableProps(1_080).columns[0]).toMatchObject({ title: 'Recorded', width: 240 });
    expect(tableProps(1_080)).toMatchObject({ pagination: { pageSize: 100 }, size: 'middle' });
    expect(screen.getByLabelText('Principal')).toBeTruthy();
    expect(screen.queryByLabelText('log.field.type')).toBeNull();

    fireEvent.click(screen.getByRole('tab', { name: 'Runtime stream' }));
    await waitFor(() => expect(screen.getByTestId('table-760')).toBeTruthy());
    expect(tableProps(760).columns.map(columnKey)).toEqual(['level', 'timestamp']);
    expect(tableProps(760).columns[0]).toMatchObject({ title: 'Severity', width: 160 });
    expect(tableProps(760)).toMatchObject({ pagination: { pageSize: 50 }, size: 'small' });
    expect(screen.getByLabelText('Contains')).toBeTruthy();
    expect(screen.getByLabelText('Minimum severity')).toBeTruthy();
    expect(screen.queryByLabelText('log.field.timeRange')).toBeNull();
    expect(screen.getByRole('link', { name: /log\.export/ })).toBeTruthy();
  });
});
