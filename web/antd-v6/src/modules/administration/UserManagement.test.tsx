import {
  ADMIN_PRESENTATION_API_VERSION,
  ADMIN_PRESENTATION_KIND,
} from '@mss-admin-core/shared/presentation/contract';
import { resolveEffectivePagePresentation } from '@mss-admin-core/shared/presentation/runtime';
import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import type { ReactNode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UserManagement from './UserManagement';
import { userPresentationRegistryEntry } from './userPresentation';

const tableRuntime = vi.hoisted(() => ({
  props: undefined as Record<string, unknown> | undefined,
}));

vi.mock('./AdministrationTable', () => ({
  AdministrationStatusTag: ({ status }: { status: string }) => <span>{status}</span>,
  default: (props: Record<string, unknown>) => {
    tableRuntime.props = props;
    return <section>{props.toolbar as ReactNode}</section>;
  },
}));

vi.mock('./query', () => ({
  useAdministrationPage: () => ({
    data: { data: [], total: 0, current: 1, pageSize: 20 },
    error: null,
    isError: false,
    isFetching: false,
    isPending: false,
  }),
  useAdministrationCatalog: () => ({
    data: [],
    error: null,
    isError: false,
    isPending: false,
  }),
}));

vi.mock('@tanstack/react-query', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-query')>();
  return {
    ...actual,
    useQueryClient: () => ({ invalidateQueries: vi.fn() }),
    useMutation: () => ({
      error: null,
      isError: false,
      isPending: false,
      mutate: vi.fn(),
      reset: vi.fn(),
    }),
  };
});

vi.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'en-US',
    formatMessage: ({ id }: { id: string }) => id,
  }),
}));

vi.mock('@mss-admin-core/shared/navigation/managementRoute', () => ({
  finishManagementRouteIntent: vi.fn(),
  useManagementRouteIntent: vi.fn(),
}));

const rootUser = {
  id: 'root-user',
  role: { root: true },
  permissions: {},
};

function compiledRuntime() {
  return resolveEffectivePagePresentation({
    entry: userPresentationRegistryEntry,
    locale: 'en-US',
    user: rootUser,
    settled: true,
  });
}

function activeRuntime() {
  const definitionHash = userPresentationRegistryEntry.definitionHash;
  return resolveEffectivePagePresentation({
    entry: userPresentationRegistryEntry,
    locale: 'en-US',
    user: rootUser,
    settled: true,
    response: {
      pageKey: 'user.list',
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
            name: 'user-management-active-test',
            pageKey: 'user.list',
            definitionHash,
            scope: { kind: 'application' },
          },
          spec: {
            list: {
              columns: [
                { field: 'email', hidden: true },
                { field: 'status', order: 5, label: { 'en-US': 'Account state' } },
              ],
              density: 'compact',
              pageSize: 50,
            },
            search: {
              collapsedByDefault: true,
              fields: [
                { field: 'name', label: { 'en-US': 'Display name' } },
                { field: 'status', hidden: true },
              ],
            },
          },
        },
      },
    },
  });
}

function columnKey(column: Record<string, unknown>): string {
  return String(column.key ?? column.dataIndex ?? '');
}

function renderManagement(
  presentationRuntime = compiledRuntime(),
  permissions = { canCreate: true, canDelete: true, canEdit: true, canResetPassword: true },
) {
  return render(
    <App>
      <UserManagement {...permissions} presentationRuntime={presentationRuntime} />
    </App>,
  );
}

describe('user management presentation consumption', () => {
  afterEach(() => {
    cleanup();
    tableRuntime.props = undefined;
  });

  it('keeps the current compiled list, filters, density, page size, and root toolbar by default', () => {
    renderManagement();

    const props = tableRuntime.props as {
      columns: Record<string, unknown>[];
      density: string;
      nameSearch: { field: string; label: string };
      params: { pageSize: number };
      resetPageSize: number;
      searchCollapsedByDefault: boolean;
      statusSearch: { field: string; label: string };
    };
    expect(props.columns.map(columnKey)).toEqual([
      'username',
      'name',
      'email',
      'roleName',
      'organization',
      'status',
      'actions',
    ]);
    expect(props).toMatchObject({
      density: 'large',
      resetPageSize: 20,
      searchCollapsedByDefault: false,
    });
    expect(props.params.pageSize).toBe(20);
    expect(props.nameSearch).toMatchObject({ field: 'name', label: 'Name' });
    expect(props.statusSearch).toMatchObject({ field: 'status', label: 'Status' });
    expect(screen.getByRole('button', { name: 'user.create.action' })).toBeTruthy();
  });

  it('applies active display facts after compiled render without expanding mutation authority', async () => {
    const view = renderManagement(compiledRuntime(), {
      canCreate: false,
      canDelete: false,
      canEdit: false,
      canResetPassword: false,
    });

    view.rerender(
      <App>
        <UserManagement
          canCreate={false}
          canDelete={false}
          canEdit={false}
          canResetPassword={false}
          presentationRuntime={activeRuntime()}
        />
      </App>,
    );

    await waitFor(() =>
      expect((tableRuntime.props as { params: { pageSize: number } }).params.pageSize).toBe(50),
    );
    const props = tableRuntime.props as {
      columns: Record<string, unknown>[];
      density: string;
      nameSearch: { field: string; label: string };
      resetPageSize: number;
      searchCollapsedByDefault: boolean;
      statusSearch: unknown;
    };
    expect(props.columns.map(columnKey)).toEqual([
      'status',
      'username',
      'name',
      'roleName',
      'organization',
      'actions',
    ]);
    expect(props.columns[0]?.title).toBe('Account state');
    expect(props).toMatchObject({
      density: 'compact',
      resetPageSize: 50,
      searchCollapsedByDefault: true,
    });
    expect(props.nameSearch).toMatchObject({ field: 'name', label: 'Display name' });
    expect(props.statusSearch).toBeNull();
    expect(screen.queryByRole('button', { name: 'user.create.action' })).toBeNull();

    const actionColumn = props.columns.find((column) => columnKey(column) === 'actions');
    const renderAction = actionColumn?.render as
      | ((value: unknown, record: Record<string, unknown>, index: number) => ReactNode)
      | undefined;
    const actionView = render(
      <App>
        {renderAction?.(undefined, { id: 'user-1', username: 'operator', status: 'enabled' }, 0)}
      </App>,
    );
    expect(actionView.queryByRole('button', { name: 'actions.edit' })).toBeNull();
    expect(actionView.queryByRole('button', { name: 'user.passwordReset.action' })).toBeNull();
    expect(actionView.queryByRole('button', { name: 'actions.delete' })).toBeNull();
  });

  it('tracks runtime page-size fallback until the user changes the query', async () => {
    const view = renderManagement();

    view.rerender(
      <App>
        <UserManagement
          canCreate
          canDelete
          canEdit
          canResetPassword
          presentationRuntime={activeRuntime()}
        />
      </App>,
    );
    await waitFor(() =>
      expect((tableRuntime.props as { params: { pageSize: number } }).params.pageSize).toBe(50),
    );

    view.rerender(
      <App>
        <UserManagement
          canCreate
          canDelete
          canEdit
          canResetPassword
          presentationRuntime={compiledRuntime()}
        />
      </App>,
    );
    await waitFor(() =>
      expect((tableRuntime.props as { params: { pageSize: number } }).params.pageSize).toBe(20),
    );

    const setParams = (tableRuntime.props as { setParams: (value: unknown) => void }).setParams;
    act(() => {
      setParams((current: { pageSize: number }) => ({ ...current, pageSize: 100 }));
    });
    view.rerender(
      <App>
        <UserManagement
          canCreate
          canDelete
          canEdit
          canResetPassword
          presentationRuntime={activeRuntime()}
        />
      </App>,
    );
    await waitFor(() =>
      expect((tableRuntime.props as { params: { pageSize: number } }).params.pageSize).toBe(100),
    );
  });

  it('keeps edit and delete disabled for a root target after presentation resolution', () => {
    renderManagement(activeRuntime());
    const props = tableRuntime.props as { columns: Record<string, unknown>[] };
    const actionColumn = props.columns.find((column) => columnKey(column) === 'actions');
    const renderAction = actionColumn?.render as
      | ((value: unknown, record: Record<string, unknown>, index: number) => ReactNode)
      | undefined;

    render(
      <App>
        {renderAction?.(
          undefined,
          {
            id: 'root-target',
            username: 'root',
            status: 'enabled',
            role: { id: 'root-role', name: 'Root', root: true },
          },
          0,
        )}
      </App>,
    );

    expect(screen.getByRole('button', { name: 'actions.edit' }).hasAttribute('disabled')).toBe(
      true,
    );
    expect(screen.getByRole('button', { name: 'actions.delete' }).hasAttribute('disabled')).toBe(
      true,
    );
    expect(
      screen.getByRole('button', { name: 'user.passwordReset.action' }).hasAttribute('disabled'),
    ).toBe(false);
  });
});
