import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { render, screen } from '@testing-library/react';
import type { PropsWithChildren } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import PresentationConfigPage from './index';

const runtime = vi.hoisted(() => ({ initialState: undefined as InitialState | undefined }));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
  useModel: () => ({ initialState: runtime.initialState }),
}));

vi.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: PropsWithChildren) => <main>{children}</main>,
}));

vi.mock('@mss-admin-core/modules/presentation-config/PresentationConfigConsole', () => ({
  default: (props: { canDraft: boolean; canPublish: boolean; canRollback: boolean }) => (
    <div>{`console:${props.canDraft}:${props.canPublish}:${props.canRollback}`}</div>
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

describe('presentation configuration route guard', () => {
  beforeEach(() => {
    runtime.initialState = undefined;
  });

  it('fails closed before identity bootstrap and without read permission', () => {
    const view = render(<PresentationConfigPage />);
    expect(screen.getByText('403')).toBeTruthy();
    expect(screen.queryByText(/^console:/)).toBeNull();
    runtime.initialState = state({});
    view.rerender(<PresentationConfigPage />);
    expect(screen.getByText('403')).toBeTruthy();
  });

  it('passes only exact workflow permissions to the console', () => {
    runtime.initialState = state({
      '/presentation-config': true,
      '/presentation-config/draft': true,
      '/presentation-config/rollback': true,
    });
    render(<PresentationConfigPage />);
    expect(screen.getByText('console:true:false:true')).toBeTruthy();
  });

  it('keeps root authoritative', () => {
    runtime.initialState = state({}, true);
    render(<PresentationConfigPage />);
    expect(screen.getByText('console:true:true:true')).toBeTruthy();
  });
});
