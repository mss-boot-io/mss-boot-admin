import { fireEvent, render, screen } from '@testing-library/react';
import * as React from 'react';
import Login, { activateOAuthLoginSession, hasAutoLoginSession, persistLoginState } from './index';
import { resolveSafeRedirect } from './redirect';
import { clearTransientAuthToken, getAuthToken, setTransientAuthToken } from '@/utils/authStorage';

jest.mock('@umijs/max', () => {
  const ReactRuntime = require('react');
  const messages: Record<string, string> = {
    'menu.login': 'Login',
    'pages.login.form.title':
      'A framework for quickly developing http/grpc services to help you quickly build monolithic services or microservice systems',
    'pages.login.password.placeholder': 'Password',
    'pages.login.username.placeholder': 'Username',
  };

  return {
    FormattedMessage: ({ defaultMessage, id }: { defaultMessage?: string; id: string }) =>
      ReactRuntime.createElement(ReactRuntime.Fragment, null, defaultMessage || messages[id] || id),
    Helmet: ({ children }: { children?: any }) =>
      ReactRuntime.createElement(ReactRuntime.Fragment, null, children),
    history: { push: jest.fn() },
    Link: ({ children }: { children?: any }) => ReactRuntime.createElement('span', null, children),
    SelectLang: () => null,
    useIntl: () => ({
      formatMessage: ({ defaultMessage, id }: { defaultMessage?: string; id: string }) =>
        messages[id] || defaultMessage || id,
    }),
    useModel: () => ({
      initialState: {
        appConfig: { base: {}, security: {} },
        fetchUserInfo: jest.fn(),
      },
      setInitialState: jest.fn(),
    }),
  };
});

jest.mock('@ant-design/use-emotion-css', () => ({
  useEmotionCss: () => 'mock-emotion-class',
}));

jest.mock('ahooks', () => ({
  useRequest: jest.fn(),
}));

jest.mock('@ant-design/icons', () => {
  const ReactRuntime = require('react');
  const Icon = () => ReactRuntime.createElement('span');

  return {
    GithubOutlined: Icon,
    LockOutlined: Icon,
    MailOutlined: Icon,
    MobileOutlined: Icon,
    UserOutlined: Icon,
  };
});

jest.mock('@ant-design/pro-components', () => {
  const ReactRuntime = require('react');
  const ProFormText: any = ({ name, placeholder }: { name?: string; placeholder?: string }) =>
    ReactRuntime.createElement('input', { name, placeholder });

  ProFormText.Password = ({ name, placeholder }: { name?: string; placeholder?: string }) =>
    ReactRuntime.createElement('input', { name, placeholder, type: 'password' });

  return {
    LoginForm: ({ children, logo, subTitle, title }: any) =>
      ReactRuntime.createElement(
        'main',
        null,
        logo,
        ReactRuntime.createElement('h1', null, title),
        ReactRuntime.createElement('p', { className: 'ant-pro-form-login-desc' }, subTitle),
        ReactRuntime.createElement('form', null, children),
      ),
    ProFormCaptcha: ({ name, placeholder }: { name?: string; placeholder?: string }) =>
      ReactRuntime.createElement('input', { name, placeholder }),
    ProFormCheckbox: ({ children, name }: { children?: any; name?: string }) =>
      ReactRuntime.createElement(
        'label',
        null,
        ReactRuntime.createElement('input', { name, type: 'checkbox' }),
        children,
      ),
    ProFormText,
  };
});

jest.mock('antd', () => {
  const ReactRuntime = require('react');

  return {
    message: { error: jest.fn(), success: jest.fn() },
    Tabs: ({ items }: { items?: Array<{ key: string; label: any }> }) =>
      ReactRuntime.createElement(
        'div',
        null,
        ...(items || []).map((item) =>
          ReactRuntime.createElement('span', { key: item.key }, item.label),
        ),
      ),
  };
});

jest.mock('@/components/Footer', () => () => null);

jest.mock('@/components/MssBoot/icon', () => ({
  LarkOutlined: () => null,
}));

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
    clearTransientAuthToken();
    localStorage.clear();
    jest.clearAllMocks();
  });

  afterEach(() => {
    jest.clearAllTimers();
  });

  it('should show the login form and accept account input', () => {
    const rootContainer = render(<Login />);

    expect(screen.getByRole('heading', { name: 'mss-boot-io' })).toBeTruthy();
    expect(rootContainer.container.querySelector('.ant-pro-form-login-desc')?.textContent).toBe(
      'A framework for quickly developing http/grpc services to help you quickly build monolithic services or microservice systems',
    );

    const userNameInput = screen.getByPlaceholderText('Username');
    const passwordInput = screen.getByPlaceholderText('Password');

    fireEvent.change(userNameInput, { target: { value: 'admin' } });
    fireEvent.change(passwordInput, { target: { value: 'ant.design' } });

    expect((userNameInput as HTMLInputElement).value).toBe('admin');
    expect((passwordInput as HTMLInputElement).value).toBe('ant.design');
  });

  it('clears a document-scoped OAuth session when browser history remounts login', () => {
    setTransientAuthToken('stale-oauth-token');

    render(<Login />);

    expect(getAuthToken({ getItem: jest.fn(() => null) })).toBeNull();
  });

  it('should persist login state and resolve redirect', () => {
    setTransientAuthToken('stale-oauth-token');

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
    expect(getAuthToken({ getItem: jest.fn(() => 'test-token') })).toBe('test-token');
    expect(redirect).toBe('/workplace');
  });

  it('keeps an OAuth login token in document memory only', () => {
    const redirect = activateOAuthLoginSession(
      'oauth-admin-token',
      'https://admin-beta.mss-boot-io.top/user/login?redirect=/workplace',
    );

    expect(getAuthToken()).toBe('oauth-admin-token');
    expect(localStorage.setItem).not.toHaveBeenCalled();
    expect(redirect).toBe('/workplace');
  });

  it('refreshes only when auto login is explicitly enabled', () => {
    const values: Record<string, string> = {
      autoLogin: 'false',
      token: 'admin-token',
      'token.expire': '3600',
    };
    const storage = { getItem: jest.fn((key: string) => values[key] || null) };

    expect(hasAutoLoginSession(storage)).toBe(false);
    values.autoLogin = 'true';
    expect(hasAutoLoginSession(storage)).toBe(true);
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
