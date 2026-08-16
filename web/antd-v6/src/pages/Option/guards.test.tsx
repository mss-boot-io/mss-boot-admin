import { render, screen } from '@testing-library/react';
import type { PropsWithChildren } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { InitialState } from '@/shared/auth/types';
import CreateOptionPage from './Create';
import EditOptionPage from './Edit';
import OptionPage from './index';

const runtime = vi.hoisted(() => ({
  initialState: undefined as InitialState | undefined,
  routeID: 'option-1',
}));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
  useModel: () => ({ initialState: runtime.initialState }),
  useParams: () => ({ id: runtime.routeID }),
}));

vi.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: PropsWithChildren) => <main>{children}</main>,
}));

vi.mock('@/modules/option/OptionListView', () => ({
  default: (props: { canCreate: boolean; canDelete: boolean; canEdit: boolean }) => (
    <div>{`list:${props.canCreate}:${props.canEdit}:${props.canDelete}`}</div>
  ),
}));

vi.mock('@/modules/option/OptionEditor', () => ({
  default: ({ id, mode }: { id?: string; mode: string }) => (
    <div>{`editor:${mode}:${id ?? ''}`}</div>
  ),
}));

function state(permissions: Record<string, boolean>, root = false): InitialState {
  return {
    currentUser: { id: 'user-1', role: { root }, permissions },
    authSessionId: 'session-1',
    settings: {},
    authorizedMenu: [],
    fetchCurrentUser: async () => undefined,
  };
}

describe('option route guards', () => {
  beforeEach(() => {
    runtime.initialState = undefined;
    runtime.routeID = 'option-1';
  });

  it('fails closed before identity bootstrap and without read permission', () => {
    const view = render(<OptionPage />);
    expect(screen.getByText('403')).toBeTruthy();
    expect(screen.queryByText(/^list:/)).toBeNull();
    runtime.initialState = state({});
    view.rerender(<OptionPage />);
    expect(screen.getByText('403')).toBeTruthy();
  });

  it('passes only exact action permissions to the list', () => {
    runtime.initialState = state({
      '/option': true,
      '/option/create': true,
      '/option/delete': true,
    });
    render(<OptionPage />);
    expect(screen.getByText('list:true:false:true')).toBeTruthy();
  });

  it('guards create and edit independently while root remains authoritative', () => {
    runtime.initialState = state({ '/option': true });
    const create = render(<CreateOptionPage />);
    expect(screen.getByText('403')).toBeTruthy();
    create.unmount();

    runtime.initialState = state({ '/option/edit': true });
    const edit = render(<EditOptionPage />);
    expect(screen.getByText('editor:edit:option-1')).toBeTruthy();
    edit.unmount();

    runtime.initialState = state({}, true);
    render(<CreateOptionPage />);
    expect(screen.getByText('editor:create:')).toBeTruthy();
  });
});
