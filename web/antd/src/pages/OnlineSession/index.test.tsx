import { render } from '@testing-library/react';
import { ProTable } from '@ant-design/pro-components';
import { useModel } from '@umijs/max';
import {
  deleteOnlineSession,
  deleteOnlineSessionUser,
  getOnlineSession,
  getOnlineSessions,
} from '@/services/admin/onlineSession';
import OnlineSessionPage from './index';

const React = require('react');

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: { children: React.ReactNode }) => children,
  ProTable: jest.fn(() => null),
}));
jest.mock('@umijs/max', () => ({
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
  useModel: jest.fn(),
}));
jest.mock('@/services/admin/onlineSession', () => ({
  deleteOnlineSession: jest.fn(),
  deleteOnlineSessionUser: jest.fn(),
  getOnlineSession: jest.fn(),
  getOnlineSessions: jest.fn(),
}));
jest.mock('./components/RevokeUserModal', () => () => null);
jest.mock('./components/SessionDetailDrawer', () => () => null);

describe('online session root-only management', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('does not mount the list, detail, or revocation actions for a delegated non-root user', () => {
    (useModel as jest.Mock).mockReturnValue({
      initialState: {
        currentUser: {
          role: { root: false },
          permissions: { '/security/online-sessions': true },
        },
      },
    });

    render(<OnlineSessionPage />);

    expect(ProTable).not.toHaveBeenCalled();
    expect(getOnlineSessions).not.toHaveBeenCalled();
    expect(getOnlineSession).not.toHaveBeenCalled();
    expect(deleteOnlineSession).not.toHaveBeenCalled();
    expect(deleteOnlineSessionUser).not.toHaveBeenCalled();
  });

  it('mounts the management table for root', () => {
    (useModel as jest.Mock).mockReturnValue({
      initialState: { currentUser: { role: { root: true } } },
    });

    render(<OnlineSessionPage />);

    expect(ProTable).toHaveBeenCalledTimes(1);
  });
});
