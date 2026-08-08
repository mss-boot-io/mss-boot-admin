import { history } from '@umijs/max';
import { message } from 'antd';
import { errorConfig } from './requestErrorConfig';
import { getAuthToken, setTransientAuthToken } from './utils/authStorage';
import { requestPermissionRefresh } from './utils/permissionFreshness';
import { THEME_AUTH_SESSION_KEY, USER_THEME_SNAPSHOT_PREFIX } from './utils/themeSession';

jest.mock('@umijs/max', () => ({
  history: { push: jest.fn() },
}));

jest.mock('antd', () => ({
  message: { error: jest.fn() },
  notification: { open: jest.fn() },
}));

jest.mock('./utils/permissionFreshness', () => ({
  ...jest.requireActual('./utils/permissionFreshness'),
  requestPermissionRefresh: jest.fn(),
}));

describe('request authentication failures', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (localStorage.getItem as jest.Mock).mockImplementation((key: string) =>
      key === THEME_AUTH_SESSION_KEY ? 'expired-theme-session' : null,
    );
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
    expect(localStorage.removeItem).toHaveBeenCalledWith(
      `${USER_THEME_SNAPSHOT_PREFIX}expired-theme-session`,
    );
    expect(localStorage.removeItem).toHaveBeenCalledWith(THEME_AUTH_SESSION_KEY);
    expect(getAuthToken({ getItem: jest.fn(() => null) })).toBeNull();
    expect(history.push).toHaveBeenCalledWith('/user/login');
  });

  it('still clears identity on a silent 401 before rethrowing', () => {
    const errorHandler = errorConfig.errorConfig?.errorHandler as (
      error: unknown,
      options: unknown,
    ) => void;
    const error = Object.assign(new Error('session expired'), {
      response: {
        status: 401,
        statusText: 'Unauthorized',
        data: { msg: 'session expired' },
      },
    });

    expect(() => errorHandler(error, { skipErrorHandler: true })).toThrow(error);

    expect(message.error).not.toHaveBeenCalled();
    expect(localStorage.removeItem).toHaveBeenCalledWith('token');
    expect(localStorage.removeItem).toHaveBeenCalledWith('token.expire');
    expect(localStorage.removeItem).toHaveBeenCalledWith('autoLogin');
    expect(localStorage.removeItem).toHaveBeenCalledWith(
      `${USER_THEME_SNAPSHOT_PREFIX}expired-theme-session`,
    );
    expect(localStorage.removeItem).toHaveBeenCalledWith(THEME_AUTH_SESSION_KEY);
    expect(getAuthToken({ getItem: jest.fn(() => null) })).toBeNull();
    expect(history.push).toHaveBeenCalledWith('/user/login');
  });

  it('throttles permission refresh requests after repeated 403 responses', () => {
    const errorHandler = errorConfig.errorConfig?.errorHandler as (
      error: unknown,
      options: unknown,
    ) => void;
    const now = jest.spyOn(Date, 'now');
    const forbidden = {
      response: {
        status: 403,
        statusText: 'Forbidden',
        data: { msg: 'permission changed' },
      },
    };

    now.mockReturnValue(1_000);
    errorHandler(forbidden, {});
    now.mockReturnValue(30_999);
    errorHandler(forbidden, {});
    expect(requestPermissionRefresh).toHaveBeenCalledTimes(1);

    now.mockReturnValue(31_000);
    errorHandler(forbidden, {});
    expect(requestPermissionRefresh).toHaveBeenCalledTimes(2);

    now.mockRestore();
  });
});
