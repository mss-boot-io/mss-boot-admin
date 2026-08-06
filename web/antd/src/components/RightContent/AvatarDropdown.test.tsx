import { render, screen } from '@testing-library/react';
import { useModel } from '@umijs/max';
import { AvatarDropdown } from './AvatarDropdown';

const React = require('react');

jest.mock('@umijs/max', () => ({
  history: { push: jest.fn(), replace: jest.fn() },
  useIntl: () => ({
    formatMessage: ({ id }: { id: string }) => id,
  }),
  useModel: jest.fn(),
}));

jest.mock('@/services/admin/onlineSession', () => ({
  postOnlineSessionLogout: jest.fn().mockResolvedValue(undefined),
}));

jest.mock('../HeaderDropdown', () => ({
  __esModule: true,
  default: ({ children }: any) => <div>{children}</div>,
}));

const mockUseModel = useModel as jest.Mock;

describe('AvatarDropdown', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('keeps a circular fallback avatar usable when the user request is unavailable', () => {
    mockUseModel.mockReturnValue({
      initialState: {},
      setInitialState: jest.fn(),
    });

    const { container } = render(
      <AvatarDropdown>
        <span data-testid="fallback-avatar" className="ant-avatar-circle" />
      </AvatarDropdown>,
    );

    expect(screen.getByTestId('fallback-avatar')).toBeTruthy();
    expect(container.querySelector('.ant-spin')).toBeNull();
  });

  it('shows a short loading state only while initial state is unresolved', () => {
    mockUseModel.mockReturnValue({
      initialState: undefined,
      setInitialState: jest.fn(),
    });

    const { container } = render(
      <AvatarDropdown>
        <span data-testid="fallback-avatar" />
      </AvatarDropdown>,
    );

    expect(container.querySelector('.ant-spin')).toBeTruthy();
    expect(screen.queryByTestId('fallback-avatar')).toBeNull();
  });
});
