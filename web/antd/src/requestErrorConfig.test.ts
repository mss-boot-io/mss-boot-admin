import { history } from '@umijs/max';
import { message } from 'antd';
import { errorConfig } from './requestErrorConfig';
import { getAuthToken, setTransientAuthToken } from './utils/authStorage';

jest.mock('@umijs/max', () => ({
  history: { push: jest.fn() },
}));

jest.mock('antd', () => ({
  message: { error: jest.fn() },
  notification: { open: jest.fn() },
}));

describe('request authentication failures', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    setTransientAuthToken('oauth-admin-token');
  });

  it('clears every auth state and redirects immediately on 401', () => {
    const errorHandler = errorConfig.errorConfig?.errorHandler as (
      error: unknown,
      options: unknown,
    ) => void;

    errorHandler(
      {
        response: {
          status: 401,
          statusText: 'Unauthorized',
          data: { msg: 'session expired' },
        },
      },
      {},
    );

    expect(message.error).toHaveBeenCalledWith('Unauthorized: session expired');
    expect(localStorage.removeItem).toHaveBeenCalledWith('token');
    expect(localStorage.removeItem).toHaveBeenCalledWith('token.expire');
    expect(localStorage.removeItem).toHaveBeenCalledWith('autoLogin');
    expect(getAuthToken({ getItem: jest.fn(() => null) })).toBeNull();
    expect(history.push).toHaveBeenCalledWith('/user/login');
  });
});
