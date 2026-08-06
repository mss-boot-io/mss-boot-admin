import { act, fireEvent, render } from '@testing-library/react';
import * as React from 'react';
import { TestBrowser } from '@@/testBrowser';
import { persistLoginState } from './index';
import { resolveSafeRedirect } from './redirect';

jest.mock('@/services/admin/appConfig', () => ({
  getAppConfigsProfile: jest.fn().mockResolvedValue({}),
}));

jest.mock('@/services/admin/user', () => ({
  getUserUserInfo: jest.fn(),
  postUserFakeCaptcha: jest.fn(),
  postUserLogin: jest.fn(),
  postUserRefreshToken: jest.fn(),
}));

describe('Login Page', () => {
  beforeEach(() => {
    localStorage.clear();
    jest.clearAllMocks();
  });

  afterEach(() => {
    jest.clearAllTimers();
  });

  it('should show login form', async () => {
    const historyRef = React.createRef<any>();
    const rootContainer = render(
      <TestBrowser
        historyRef={historyRef}
        location={{
          pathname: '/user/login',
        }}
      />,
    );

    await rootContainer.findAllByText('mss-boot-io');

    act(() => {
      historyRef.current?.push('/user/login');
    });

    expect(rootContainer.baseElement?.querySelector('.ant-pro-form-login-desc')?.textContent).toBe(
      'A framework for quickly developing http/grpc services to help you quickly build monolithic services or microservice systems',
    );

    expect(rootContainer.asFragment()).toMatchSnapshot();

    rootContainer.unmount();
  });

  it('should accept account input', async () => {
    const historyRef = React.createRef<any>();
    const rootContainer = render(
      <TestBrowser
        historyRef={historyRef}
        location={{
          pathname: '/user/login',
        }}
      />,
    );

    await rootContainer.findAllByText('mss-boot-io');

    const userNameInput = await rootContainer.findByPlaceholderText('Username');

    act(() => {
      fireEvent.change(userNameInput, { target: { value: 'admin' } });
    });

    const passwordInput = await rootContainer.findByPlaceholderText('Password');

    act(() => {
      fireEvent.change(passwordInput, { target: { value: 'ant.design' } });
    });

    expect((userNameInput as HTMLInputElement).value).toBe('admin');
    expect((passwordInput as HTMLInputElement).value).toBe('ant.design');

    rootContainer.unmount();
  });

  it('should persist login state and resolve redirect', () => {
    const redirect = persistLoginState(
      {
        code: 200,
        token: 'test-token',
        expire: '3600',
      },
      true,
      'https://admin-beta.mss-boot-io.top/user/login?redirect=/workplace',
    );

    expect(localStorage.setItem).toHaveBeenCalledWith('token', 'test-token');
    expect(localStorage.setItem).toHaveBeenCalledWith('token.expire', '3600');
    expect(localStorage.setItem).toHaveBeenCalledWith('autoLogin', 'true');
    expect(redirect).toBe('/workplace');
  });

  it('should reject unsafe login redirects', () => {
    expect(
      resolveSafeRedirect(
        'https://admin-beta.mss-boot-io.top/user/login?redirect=https%3A%2F%2Fevil.example%2Fworkplace',
      ),
    ).toBe('/');
    expect(
      resolveSafeRedirect(
        'https://admin-beta.mss-boot-io.top/user/login?redirect=javascript%3Aalert(1)',
      ),
    ).toBe('/');
    expect(
      resolveSafeRedirect(
        'https://admin-beta.mss-boot-io.top/user/login?redirect=%2F%2Fevil.example%2Fworkplace',
      ),
    ).toBe('/');
  });

  it('should keep same-origin login redirects as paths', () => {
    expect(
      resolveSafeRedirect(
        'https://admin-beta.mss-boot-io.top/user/login?redirect=https%3A%2F%2Fadmin-beta.mss-boot-io.top%2Fapp-config%3Ftab%3Dbase%23safe',
      ),
    ).toBe('/app-config?tab=base#safe');
  });
});
