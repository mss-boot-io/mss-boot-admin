import ApiOutlined from '@ant-design/icons/ApiOutlined';
import GithubOutlined from '@ant-design/icons/GithubOutlined';
import LockOutlined from '@ant-design/icons/LockOutlined';
import MailOutlined from '@ant-design/icons/MailOutlined';
import SafetyCertificateOutlined from '@ant-design/icons/SafetyCertificateOutlined';
import UserOutlined from '@ant-design/icons/UserOutlined';
import { LoginFormPage, ProFormCaptcha, ProFormText } from '@ant-design/pro-components';
import { history, Link, SelectLang, useIntl, useModel } from '@umijs/max';
import { Alert, App, Button, Divider, Space, Tabs, Typography } from 'antd';
import { useState } from 'react';
import { accountAPI } from '@/modules/account/api';
import type { OAuthProvider } from '@/modules/account/contracts';
import { authAPI } from '@/modules/auth/api';
import { emailChallengeCapability, isEmailChallengeFlowAvailable } from '@/modules/auth/capability';
import { rememberOAuthLoginRedirect } from '@/shared/auth/oauthRedirect';
import { resolveSafeRedirect } from '@/shared/auth/redirect';
import {
  clearStaleBrowserAuthCookie,
  createBrowserSession,
  fetchCurrentUser,
} from '@/shared/auth/session';
import type { InitialState } from '@/shared/auth/types';
import { isOAuthProviderEnabled } from '@/shared/theme/contract';
import { rotateThemeAuthSession } from '@/shared/theme/snapshot';

export default function LoginPage() {
  const intl = useIntl();
  const { message } = App.useApp();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const [error, setError] = useState<string>();
  const [loginType, setLoginType] = useState<'account' | 'email'>('account');
  const [oauthProvider, setOAuthProvider] = useState<OAuthProvider>();
  const emailCapability = emailChallengeCapability(initialState?.applicationProfile);
  const emailLoginAvailable = isEmailChallengeFlowAvailable(
    initialState?.applicationProfile,
    'login',
  );
  const registrationAvailable = isEmailChallengeFlowAvailable(
    initialState?.applicationProfile,
    'register',
  );
  const githubEnabled = isOAuthProviderEnabled(initialState?.applicationProfile, 'github');
  const larkEnabled = isOAuthProviderEnabled(initialState?.applicationProfile, 'lark');
  const brandTitle =
    typeof initialState?.settings.title === 'string' && initialState.settings.title.trim()
      ? initialState.settings.title
      : 'mss-boot-io';
  const brandLogo =
    typeof initialState?.settings.logo === 'string' && initialState.settings.logo.trim()
      ? initialState.settings.logo
      : '/logo.svg';

  const startOAuthLogin = async (provider: OAuthProvider) => {
    if (oauthProvider) return;
    setError(undefined);
    setOAuthProvider(provider);
    try {
      // Login authorization must start anonymously. Expire a residual cookie
      // first so OptionalAuth cannot turn this into an authenticated conflict.
      await clearStaleBrowserAuthCookie().catch(() => undefined);
      const attempt = await accountAPI.startOAuthAuthorization(provider, 'login');
      const params = new URLSearchParams(history.location.search);
      rememberOAuthLoginRedirect(attempt.attemptID, attempt.expiresAt, params.get('redirect'));
      window.location.assign(attempt.authorizeURL);
    } catch {
      setError(intl.formatMessage({ id: 'pages.login.oauthFailure' }));
      setOAuthProvider(undefined);
    }
  };

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top_left,var(--mss-color-primary-bg),transparent_42%),var(--mss-color-bg-layout)] px-4 py-10">
      <div className="fixed right-4 top-4 z-10 rounded-lg bg-[var(--mss-color-bg-container)] shadow-sm">
        <SelectLang />
      </div>
      <LoginFormPage
        logo={brandLogo}
        title={brandTitle}
        subTitle={intl.formatMessage({ id: 'pages.login.subtitle' })}
        backgroundImageUrl=""
        containerStyle={{
          background: 'color-mix(in srgb, var(--mss-color-bg-container) 94%, transparent)',
          backdropFilter: 'blur(18px)',
          border: '1px solid var(--mss-color-border-secondary)',
          borderRadius: 16,
          boxShadow: 'var(--mss-box-shadow-secondary)',
        }}
        onFinish={async (values) => {
          setError(undefined);
          try {
            const result =
              loginType === 'email'
                ? await createBrowserSession({
                    type: 'email',
                    email: values.email,
                    captcha: values.captcha,
                  })
                : await createBrowserSession({
                    username: values.username,
                    password: values.password,
                  });
            if (result.code && result.code !== 200) {
              setError(intl.formatMessage({ id: 'pages.login.failure' }));
              return false;
            }
            const currentUser = await fetchCurrentUser();
            if (!currentUser) throw new Error('Session identity was not established');
            rotateThemeAuthSession(currentUser.id);
            await message.success(intl.formatMessage({ id: 'pages.login.success' }));
            const params = new URLSearchParams(history.location.search);
            window.location.replace(resolveSafeRedirect(params.get('redirect')));
            return true;
          } catch {
            setError(intl.formatMessage({ id: 'pages.login.failure' }));
            return false;
          }
        }}
      >
        {error ? <Alert className="mb-4" type="error" showIcon title={error} /> : null}
        <Tabs
          className="mb-4"
          activeKey={loginType}
          onChange={(key) => {
            setError(undefined);
            setLoginType(key as 'account' | 'email');
          }}
          items={[
            {
              key: 'account',
              label: intl.formatMessage({ id: 'pages.login.accountTab' }),
            },
            ...(emailLoginAvailable
              ? [
                  {
                    key: 'email',
                    label: intl.formatMessage({ id: 'pages.login.emailTab' }),
                  },
                ]
              : []),
          ]}
        />
        {emailCapability.emailEnabled && !emailCapability.ready ? (
          <Alert
            className="mb-4"
            type="warning"
            showIcon
            title={intl.formatMessage({ id: 'pages.auth.emailUnavailable' })}
          />
        ) : null}
        {loginType === 'account' ? (
          <>
            <ProFormText
              name="username"
              fieldProps={{
                autoComplete: 'username',
                prefix: <UserOutlined />,
              }}
              placeholder={intl.formatMessage({ id: 'pages.login.username' })}
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({ id: 'pages.login.usernameRequired' }),
                },
              ]}
            />
            <ProFormText.Password
              name="password"
              fieldProps={{
                autoComplete: 'current-password',
                prefix: <LockOutlined />,
              }}
              placeholder={intl.formatMessage({ id: 'pages.login.password' })}
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({ id: 'pages.login.passwordRequired' }),
                },
              ]}
            />
          </>
        ) : (
          <>
            <ProFormText
              name="email"
              fieldProps={{ autoComplete: 'email', prefix: <MailOutlined /> }}
              placeholder={intl.formatMessage({ id: 'pages.auth.email' })}
              rules={[
                { required: true, message: intl.formatMessage({ id: 'pages.auth.emailRequired' }) },
                { type: 'email', message: intl.formatMessage({ id: 'pages.auth.emailInvalid' }) },
              ]}
            />
            <ProFormCaptcha
              name="captcha"
              phoneName="email"
              fieldProps={{ autoComplete: 'one-time-code', prefix: <LockOutlined /> }}
              placeholder={intl.formatMessage({ id: 'pages.auth.captcha' })}
              captchaTextRender={(timing, count) =>
                timing
                  ? intl.formatMessage({ id: 'pages.auth.captchaCountdown' }, { count })
                  : intl.formatMessage({ id: 'pages.auth.sendCaptcha' })
              }
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({ id: 'pages.auth.captchaRequired' }),
                },
              ]}
              onGetCaptcha={async (email) => {
                await authAPI.sendEmailChallenge({ email, useBy: 'login' });
                void message.success(intl.formatMessage({ id: 'pages.auth.captchaSent' }));
              }}
            />
          </>
        )}
        <div className="mb-4 flex items-center justify-between gap-4 text-sm">
          {emailLoginAvailable ? (
            <Link to="/user/forget">{intl.formatMessage({ id: 'pages.login.forgot' })}</Link>
          ) : (
            <span />
          )}
          {registrationAvailable ? (
            <Link to="/user/register">{intl.formatMessage({ id: 'pages.login.register' })}</Link>
          ) : null}
        </div>
        <Typography.Paragraph className="mb-6 text-sm" type="secondary">
          {intl.formatMessage({ id: 'pages.login.sessionNotice' })}
        </Typography.Paragraph>
        {githubEnabled || larkEnabled ? (
          <>
            <Divider plain>{intl.formatMessage({ id: 'pages.login.oauthDivider' })}</Divider>
            <Space orientation="vertical" className="w-full">
              {githubEnabled ? (
                <Button
                  block
                  icon={<GithubOutlined />}
                  loading={oauthProvider === 'github'}
                  disabled={Boolean(oauthProvider && oauthProvider !== 'github')}
                  onClick={() => void startOAuthLogin('github')}
                >
                  {intl.formatMessage({ id: 'pages.login.github' })}
                </Button>
              ) : null}
              {larkEnabled ? (
                <Button
                  block
                  icon={<ApiOutlined />}
                  loading={oauthProvider === 'lark'}
                  disabled={Boolean(oauthProvider && oauthProvider !== 'lark')}
                  onClick={() => void startOAuthLogin('lark')}
                >
                  {intl.formatMessage({ id: 'pages.login.lark' })}
                </Button>
              ) : null}
            </Space>
          </>
        ) : null}
        <Space className="mt-4 text-xs text-neutral-500">
          <SafetyCertificateOutlined />
          <span>{intl.formatMessage({ id: 'pages.login.cookieSecurity' })}</span>
        </Space>
      </LoginFormPage>
    </div>
  );
}
