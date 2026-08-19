import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { render, screen } from '@testing-library/react';
import type { PropsWithChildren } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import OnlineSessionsPage from './index';

const model = vi.hoisted(() => ({ initialState: undefined as InitialState | undefined }));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
  useModel: () => ({ initialState: model.initialState }),
}));

vi.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: PropsWithChildren) => <main>{children}</main>,
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
    expect(screen.queryByText('403')).toBeNull();
  });
});
