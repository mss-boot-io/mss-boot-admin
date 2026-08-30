import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { render, screen } from '@testing-library/react';
import type { PropsWithChildren, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import CreateOptionPage from './Create';
import EditOptionPage from './Edit';
import OptionPage from './index';

const runtime = vi.hoisted(() => ({
  initialState: undefined as InitialState | undefined,
  routeID: 'option-1',
}));
const presentation = vi.hoisted(() => ({
  usePagePresentation: vi.fn(() => ({ model: { title: 'Configured options' } })),
}));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ locale: 'en-US', formatMessage: ({ id }: { id: string }) => id }),
  useModel: () => ({ initialState: runtime.initialState }),
  useParams: () => ({ id: runtime.routeID }),
}));

vi.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children, title }: PropsWithChildren<{ title?: ReactNode }>) => (
    <main>
      {title}
      {children}
    </main>
  ),
}));

vi.mock('@mss-admin-core/shared/presentation/runtime', () => ({
  usePagePresentation: presentation.usePagePresentation,
}));

vi.mock('@mss-admin-core/modules/option/OptionListView', () => ({
  default: (props: { canCreate: boolean; canDelete: boolean; canEdit: boolean }) => (
    <div>{`list:${props.canCreate}:${props.canEdit}:${props.canDelete}`}</div>
  ),
}));

vi.mock('@mss-admin-core/modules/option/OptionEditor', () => ({
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
    presentation.usePagePresentation.mockClear();
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
    expect(screen.getByRole('heading', { name: 'Configured options' })).toBeTruthy();
    expect(presentation.usePagePresentation).toHaveBeenCalledWith(
      expect.objectContaining({ definition: expect.objectContaining({ pageKey: 'option.list' }) }),
      'en-US',
      runtime.initialState?.currentUser,
      runtime.initialState?.authorizationVersion,
    );
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
