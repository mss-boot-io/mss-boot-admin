import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { render, screen } from '@testing-library/react';
import type { PropsWithChildren, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import OnlineSessionsPage from './index';

const model = vi.hoisted(() => ({ initialState: undefined as InitialState | undefined }));
const presentation = vi.hoisted(() => ({
  usePagePresentation: vi.fn(() => ({ model: { title: 'Configured sessions' } })),
}));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ locale: 'en-US', formatMessage: ({ id }: { id: string }) => id }),
  useModel: () => ({ initialState: model.initialState }),
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

vi.mock('@mss-admin-core/modules/session/OnlineSessionsView', () => ({
  default: () => <div>online-session-content</div>,
}));

function state(root: boolean): InitialState {
  return {
    currentUser: { id: root ? 'root' : 'operator', role: { root }, permissions: {} },
    authSessionId: 'session-1',
    settings: {},
    authorizedMenu: [],
    fetchCurrentUser: async () => undefined,
  };
}

describe('online sessions route guard', () => {
  beforeEach(() => {
    model.initialState = undefined;
    presentation.usePagePresentation.mockClear();
  });

  it('fails closed before identity bootstrap and for a non-root identity', () => {
    const view = render(<OnlineSessionsPage />);
    expect(screen.getByText('403')).toBeTruthy();
    expect(screen.queryByText('online-session-content')).toBeNull();

    model.initialState = state(false);
    view.rerender(<OnlineSessionsPage />);
    expect(screen.getByText('403')).toBeTruthy();
    expect(screen.queryByText('online-session-content')).toBeNull();
  });

  it('renders the compiled page only for a root identity', () => {
    model.initialState = state(true);
    render(<OnlineSessionsPage />);

    expect(screen.getByText('online-session-content')).toBeTruthy();
    expect(screen.getByRole('heading', { name: 'Configured sessions' })).toBeTruthy();
    expect(presentation.usePagePresentation).toHaveBeenCalledWith(
      expect.objectContaining({
        definition: expect.objectContaining({ pageKey: 'online-session.list' }),
      }),
      'en-US',
      model.initialState?.currentUser,
      model.initialState?.authorizationVersion,
    );
    expect(screen.queryByText('403')).toBeNull();
  });
});
