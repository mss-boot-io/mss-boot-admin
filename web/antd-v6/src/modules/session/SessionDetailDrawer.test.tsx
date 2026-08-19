import { render, screen } from '@testing-library/react';
import { App } from 'antd';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { OnlineSession } from './contract';
import SessionDetailDrawer from './SessionDetailDrawer';

const detailQuery = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'en-US',
    formatMessage: ({ id }: { id: string }) => id,
  }),
}));

vi.mock('./query', () => ({
  useOnlineSession: () => detailQuery.current,
}));

vi.mock('@mss-admin-core/modules/administration/query', () => ({
  useAdministrationPage: (resource: string) => ({
    data: {
      data:
        resource === 'roles'
          ? [{ id: 'role-1', name: 'Administrator' }]
          : [{ id: 'user-2', name: 'Security Operator', username: 'operator' }],
    },
  }),
}));

function session(): OnlineSession {
  return {
    id: 'session-1',
    userID: 'user-1',
    username: 'alice',
    roleID: 'role-1',
    loginAt: '2026-08-15T00:00:00Z',
    lastSeenAt: '2026-08-15T01:00:00Z',
    expiredAt: '2099-08-15T02:00:00Z',
    revoked: false,
    revokedBy: 'user-2',
    current: false,
  };
}

function renderDrawer() {
  return render(
    <App>
      <SessionDetailDrawer id="session-1" open onClose={vi.fn()} />
    </App>,
  );
}

describe('online session detail drawer', () => {
  beforeEach(() => {
    detailQuery.current = {
      data: undefined,
      error: null,
      isError: false,
      isPending: false,
      refetch: vi.fn(),
    };
  });

  it('renders permission denial without exposing cached details after access is forbidden', () => {
    detailQuery.current = {
      ...detailQuery.current,
      data: session(),
      error: { response: { status: 403 } },
      isError: true,
    };

    renderDrawer();

    expect(screen.getByText('403')).toBeTruthy();
    expect(screen.queryByText('alice')).toBeNull();
  });

  it('keeps last-good details visible and labels a failed refresh', () => {
    detailQuery.current = {
      ...detailQuery.current,
      data: session(),
      error: new Error('temporary'),
      isError: true,
    };

    renderDrawer();

    expect(screen.getByText('sessions.detail.refreshFailed')).toBeTruthy();
    expect(screen.getByText('alice')).toBeTruthy();
    expect(screen.getByText('Administrator')).toBeTruthy();
    expect(screen.getByText('Security Operator')).toBeTruthy();
    expect(screen.queryByText('role-1')).toBeNull();
    expect(screen.queryByText('user-2')).toBeNull();
  });

  it('labels the current session and presents a readable device summary', () => {
    detailQuery.current = {
      ...detailQuery.current,
      data: {
        ...session(),
        current: true,
        userAgent:
          'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/140.0 Safari/537.36',
      },
    };

    renderDrawer();

    expect(screen.getByText('sessions.current')).toBeTruthy();
    expect(screen.getByText('Google Chrome · Windows')).toBeTruthy();
  });
});
