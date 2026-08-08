import { fireEvent, render, screen } from '@testing-library/react';
import { history, useIntl } from '@umijs/max';
import ForbiddenPage from './403';

const React = require('react');

jest.mock('@umijs/max', () => ({
  history: { push: jest.fn() },
  useIntl: jest.fn(),
}));

describe('ForbiddenPage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (useIntl as jest.Mock).mockReturnValue({
      formatMessage: ({ id }: { id: string }) => id,
    });
  });

  it('uses an authenticated self-service action instead of looping through /welcome', () => {
    render(<ForbiddenPage />);

    expect(screen.getByText('403')).toBeTruthy();
    expect(screen.getByText('pages.403.description')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'pages.403.action.account' }));
    expect(history.push).toHaveBeenCalledWith('/account/settings');
  });
});
