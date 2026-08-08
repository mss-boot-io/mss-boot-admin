import { fireEvent, render, screen } from '@testing-library/react';
import { history } from '@umijs/max';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import MobileUserList from './UserList';

const React = require('react');

jest.mock('@umijs/max', () => ({
  history: { push: jest.fn() },
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
}));

jest.mock('@/components/MssBoot/Access', () => ({
  Access: ({ children }: any) => children,
}));

jest.mock('@/hooks/useMobileListPagination', () => ({
  useMobileListPagination: jest.fn(),
}));

jest.mock('@/services/admin/user', () => ({
  deleteUsersId: jest.fn(),
  getUsers: jest.fn(),
}));

describe('MobileUserList action parity', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (useMobileListPagination as jest.Mock).mockReturnValue({
      dataSource: [
        {
          id: 'user-1',
          username: 'operator',
          name: 'Operator',
          status: 'enabled',
          role: { root: false },
        },
      ],
      error: undefined,
      hasMore: false,
      loading: false,
      loadingMore: false,
      loadMore: jest.fn(),
      reload: jest.fn(),
    });
  });

  it('opens the existing password reset flow from a user card', () => {
    render(<MobileUserList />);

    fireEvent.click(screen.getByText('pages.title.password.reset'));

    expect(history.push).toHaveBeenCalledWith('/users/password-reset/user-1/');
  });

  it('keeps the confirmed-empty state hidden during the initial request', () => {
    (useMobileListPagination as jest.Mock).mockReturnValue({
      dataSource: [],
      error: undefined,
      hasMore: false,
      loading: true,
      loadingMore: false,
      loadMore: jest.fn(),
      reload: jest.fn(),
    });

    render(<MobileUserList />);

    expect(screen.queryByText('pages.user.mobile.empty')).toBeNull();
  });

  it('shows a retryable error instead of the confirmed-empty state after an initial failure', () => {
    const reload = jest.fn();
    (useMobileListPagination as jest.Mock).mockReturnValue({
      dataSource: [],
      error: new Error('offline'),
      hasMore: false,
      loading: false,
      loadingMore: false,
      loadMore: jest.fn(),
      reload,
    });

    render(<MobileUserList />);

    expect(screen.getByText('pages.mobile.loadFailed')).toBeTruthy();
    expect(screen.queryByText('pages.user.mobile.empty')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'pages.mobile.retry' }));
    expect(reload).toHaveBeenCalledTimes(1);
  });
});
