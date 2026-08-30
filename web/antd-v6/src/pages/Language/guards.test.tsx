import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { render, screen } from '@testing-library/react';
import type { PropsWithChildren, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import CreateLanguagePage from './Create';
import EditLanguagePage from './Edit';
import LanguagePage from './index';

const runtime = vi.hoisted(() => ({
  initialState: undefined as InitialState | undefined,
  routeID: 'language-1',
}));
const presentation = vi.hoisted(() => ({
  usePagePresentation: vi.fn(() => ({ model: { title: 'Configured languages' } })),
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

vi.mock('@mss-admin-core/modules/language/LanguageListView', () => ({
  default: (props: { canCreate: boolean; canDelete: boolean; canEdit: boolean }) => (
    <div>{`list:${props.canCreate}:${props.canEdit}:${props.canDelete}`}</div>
  ),
}));

vi.mock('@mss-admin-core/modules/language/LanguageEditor', () => ({
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

describe('language route guards', () => {
  beforeEach(() => {
    runtime.initialState = undefined;
    runtime.routeID = 'language-1';
    presentation.usePagePresentation.mockClear();
  });

  it('fails closed before identity bootstrap and without read permission', () => {
    const view = render(<LanguagePage />);
    expect(screen.getByText('403')).toBeTruthy();
    expect(screen.queryByText(/^list:/)).toBeNull();

    runtime.initialState = state({});
    view.rerender(<LanguagePage />);
    expect(screen.getByText('403')).toBeTruthy();
  });

  it('passes only exact action permissions to the list', () => {
    runtime.initialState = state({
      '/language': true,
      '/language/create': true,
      '/language/edit': true,
    });
    render(<LanguagePage />);
    expect(screen.getByText('list:true:true:false')).toBeTruthy();
    expect(screen.getByRole('heading', { name: 'Configured languages' })).toBeTruthy();
    expect(presentation.usePagePresentation).toHaveBeenCalledWith(
      expect.objectContaining({
        definition: expect.objectContaining({ pageKey: 'language.list' }),
      }),
      'en-US',
      runtime.initialState?.currentUser,
      runtime.initialState?.authorizationVersion,
    );
  });

  it('guards create and edit independently while root remains authoritative', () => {
    runtime.initialState = state({ '/language': true });
    const create = render(<CreateLanguagePage />);
    expect(screen.getByText('403')).toBeTruthy();
    create.unmount();

    runtime.initialState = state({ '/language/edit': true });
    const edit = render(<EditLanguagePage />);
    expect(screen.getByText('editor:edit:language-1')).toBeTruthy();
    edit.unmount();

    runtime.initialState = state({}, true);
    render(<CreateLanguagePage />);
    expect(screen.getByText('editor:create:')).toBeTruthy();
  });
});
