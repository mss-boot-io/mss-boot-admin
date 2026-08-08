import * as React from 'react';
import { render, screen } from '@testing-library/react';
import SecurityView from './security';

let mockInitialStateLoading = false;
let mockCurrentUser: { email?: string; phone?: string } | undefined = {
  email: 'user@example.com',
  phone: '13800000000',
};

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: ({ id }: { id: string }) => id,
  }),
  useModel: () => ({
    initialState: {
      currentUser: mockCurrentUser,
    },
    loading: mockInitialStateLoading,
  }),
}));

jest.mock('ahooks', () => ({
  useRequest: () => ({ data: undefined, loading: false }),
}));

jest.mock('@/services/admin/user', () => ({
  getUserUserInfo: jest.fn(),
  postUserResetPassword: jest.fn(),
}));

describe('SecurityView', () => {
  beforeEach(() => {
    mockInitialStateLoading = false;
    mockCurrentUser = {
      email: 'user@example.com',
      phone: '13800000000',
    };
  });

  it('keeps the implemented password action and hides unimplemented phone and email actions', () => {
    render(<SecurityView />);

    expect(screen.getByText('pages.security.settings.phoneBound')).toBeTruthy();
    expect(screen.getByText('pages.security.settings.emailBound')).toBeTruthy();
    expect(
      screen.getByRole('button', { name: 'pages.security.settings.modify' }),
    ).toBeTruthy();
    expect(screen.queryByText('pages.security.settings.bind')).toBeNull();
  });

  it('does not report phone or email as unbound while account data is loading', () => {
    mockInitialStateLoading = true;
    mockCurrentUser = undefined;

    const { container } = render(<SecurityView />);

    expect(screen.queryByText('pages.security.settings.phoneUnbound')).toBeNull();
    expect(screen.queryByText('pages.security.settings.emailUnbound')).toBeNull();
    expect(container.querySelectorAll('.ant-skeleton').length).toBeGreaterThanOrEqual(2);
  });
});
