import { LockOutlined, SafetyCertificateOutlined, UserOutlined } from '@ant-design/icons';
import { LoginFormPage, ProFormText } from '@ant-design/pro-components';
import { history, useIntl } from '@umijs/max';
import { Alert, App, Space, Typography } from 'antd';
import { useState } from 'react';
import { resolveSafeRedirect } from '@/shared/auth/redirect';
import { createBrowserSession, fetchCurrentUser } from '@/shared/auth/session';
import { rotateThemeAuthSession } from '@/shared/theme/snapshot';

export default function LoginPage() {
  const intl = useIntl();
  const { message } = App.useApp();
  const [error, setError] = useState<string>();

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top_left,var(--mss-color-primary-bg),transparent_42%),var(--mss-color-bg-layout)] px-4 py-10">
      <LoginFormPage
        logo="/logo.svg"
        title="MSS Admin"
        subTitle={intl.formatMessage({
          id: 'pages.login.subtitle',
          defaultMessage: '安全、可升级的管理系统基础设施',
        })}
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
            const result = await createBrowserSession({
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
        {error ? <Alert className="mb-4" type="error" showIcon message={error} /> : null}
        <ProFormText
          name="username"
          fieldProps={{
            autoComplete: 'username',
            prefix: <UserOutlined />,
          }}
          placeholder={intl.formatMessage({ id: 'pages.login.username' })}
          rules={[
            { required: true, message: intl.formatMessage({ id: 'pages.login.usernameRequired' }) },
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
            { required: true, message: intl.formatMessage({ id: 'pages.login.passwordRequired' }) },
          ]}
        />
        <Typography.Paragraph className="mb-6 text-sm" type="secondary">
          {intl.formatMessage({ id: 'pages.login.sessionNotice' })}
        </Typography.Paragraph>
        <Space className="mt-4 text-xs text-neutral-500">
          <SafetyCertificateOutlined />
          <span>{intl.formatMessage({ id: 'pages.login.cookieSecurity' })}</span>
        </Space>
      </LoginFormPage>
    </div>
  );
}
