import LockOutlined from '@ant-design/icons/LockOutlined';
import MailOutlined from '@ant-design/icons/MailOutlined';
import {
  ProFormCaptcha,
  type ProFormInstance,
  ProFormText,
  StepsForm,
} from '@ant-design/pro-components';
import { resolveSafeRedirect } from '@mss-admin-core/shared/auth/redirect';
import { createBrowserSession, fetchCurrentUser } from '@mss-admin-core/shared/auth/session';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { rotateThemeAuthSession } from '@mss-admin-core/shared/theme/snapshot';
import { history, useIntl, useModel } from '@umijs/max';
import { Alert, App, Result } from 'antd';
import { useRef, useState } from 'react';
import { authAPI } from './api';
import {
  type EmailChallengeFlow,
  emailChallengeCapability,
  isEmailChallengeFlowAvailable,
} from './capability';
import PublicAuthFrame from './PublicAuthFrame';

interface ChallengeValues {
  email: string;
  captcha: string;
  password: string;
  confirm: string;
}

const emailRule = { type: 'email' as const };

export default function EmailChallengeFlowPage({
  flow,
}: {
  flow: Extract<EmailChallengeFlow, 'register' | 'resetPassword'>;
}) {
  const intl = useIntl();
  const { message } = App.useApp();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const formRef = useRef<ProFormInstance | null>(null);
  const [error, setError] = useState<string>();
  const profile = initialState?.applicationProfile;
  const capability = emailChallengeCapability(profile);
  const available = isEmailChallengeFlowAvailable(profile, flow);
  const registration = flow === 'register';
  const title = intl.formatMessage({
    id: registration ? 'pages.auth.register.title' : 'pages.auth.recovery.title',
  });

  if (!available) {
    const reason =
      registration && !capability.registerEnabled
        ? 'pages.auth.registrationDisabled'
        : 'pages.auth.emailUnavailable';
    return (
      <PublicAuthFrame title={title}>
        <Result status="warning" title={intl.formatMessage({ id: reason })} />
      </PublicAuthFrame>
    );
  }

  const finish = async (values: ChallengeValues): Promise<boolean> => {
    setError(undefined);
    try {
      if (registration) {
        await createBrowserSession({
          type: 'email_register',
          email: values.email,
          captcha: values.captcha,
          password: values.password,
        });
        const currentUser = await fetchCurrentUser();
        if (!currentUser) throw new Error('Registration did not establish a browser session');
        rotateThemeAuthSession(currentUser.id);
        await message.success(intl.formatMessage({ id: 'pages.auth.register.success' }));
        const params = new URLSearchParams(history.location.search);
        window.location.replace(resolveSafeRedirect(params.get('redirect')));
        return true;
      }

      await authAPI.resetPassword({
        email: values.email,
        captcha: values.captcha,
        password: values.password,
      });
      await message.success(intl.formatMessage({ id: 'pages.auth.recovery.success' }));
      history.replace('/user/login');
      return true;
    } catch {
      setError(
        intl.formatMessage({
          id: registration ? 'pages.auth.register.failure' : 'pages.auth.recovery.failure',
        }),
      );
      return false;
    }
  };

  return (
    <PublicAuthFrame
      title={title}
      description={intl.formatMessage({
        id: registration ? 'pages.auth.register.description' : 'pages.auth.recovery.description',
      })}
    >
      {error ? <Alert className="mb-6" type="error" showIcon title={error} /> : null}
      <StepsForm<ChallengeValues>
        formRef={formRef}
        onFinish={finish}
        containerStyle={{ width: '100%' }}
        formProps={{ layout: 'vertical', requiredMark: false }}
      >
        <StepsForm.StepForm
          name="verify-email"
          title={intl.formatMessage({ id: 'pages.auth.verifyEmail' })}
        >
          <ProFormText
            name="email"
            label={intl.formatMessage({ id: 'pages.auth.email' })}
            fieldProps={{ autoComplete: 'email', prefix: <MailOutlined /> }}
            rules={[
              { required: true, message: intl.formatMessage({ id: 'pages.auth.emailRequired' }) },
              { ...emailRule, message: intl.formatMessage({ id: 'pages.auth.emailInvalid' }) },
            ]}
          />
          <ProFormCaptcha
            name="captcha"
            phoneName="email"
            label={intl.formatMessage({ id: 'pages.auth.captcha' })}
            fieldProps={{ autoComplete: 'one-time-code', prefix: <LockOutlined /> }}
            captchaTextRender={(timing, count) =>
              timing
                ? intl.formatMessage({ id: 'pages.auth.captchaCountdown' }, { count })
                : intl.formatMessage({ id: 'pages.auth.sendCaptcha' })
            }
            rules={[
              { required: true, message: intl.formatMessage({ id: 'pages.auth.captchaRequired' }) },
            ]}
            onGetCaptcha={async (email) => {
              await authAPI.sendEmailChallenge({ email, useBy: flow });
              void message.success(intl.formatMessage({ id: 'pages.auth.captchaSent' }));
            }}
          />
        </StepsForm.StepForm>
        <StepsForm.StepForm
          name="set-password"
          title={intl.formatMessage({
            id: registration ? 'pages.auth.createAccount' : 'pages.auth.setPassword',
          })}
        >
          <ProFormText.Password
            name="password"
            label={intl.formatMessage({ id: 'pages.auth.newPassword' })}
            fieldProps={{ autoComplete: 'new-password', prefix: <LockOutlined /> }}
            rules={[
              {
                required: true,
                message: intl.formatMessage({ id: 'pages.auth.passwordRequired' }),
              },
              {
                min: 8,
                max: 128,
                message: intl.formatMessage({ id: 'pages.auth.passwordLength' }),
              },
              {
                pattern: /[A-Za-z]/,
                message: intl.formatMessage({ id: 'pages.auth.passwordLetter' }),
              },
              {
                pattern: /[0-9]/,
                message: intl.formatMessage({ id: 'pages.auth.passwordNumber' }),
              },
            ]}
          />
          <ProFormText.Password
            name="confirm"
            dependencies={['password']}
            label={intl.formatMessage({ id: 'pages.auth.confirmPassword' })}
            fieldProps={{ autoComplete: 'new-password', prefix: <LockOutlined /> }}
            rules={[
              { required: true, message: intl.formatMessage({ id: 'pages.auth.confirmRequired' }) },
              ({ getFieldValue }) => ({
                validator: async (_, value) => {
                  if (!value || getFieldValue('password') === value) return;
                  throw new Error(intl.formatMessage({ id: 'pages.auth.passwordMismatch' }));
                },
              }),
            ]}
          />
        </StepsForm.StepForm>
      </StepsForm>
    </PublicAuthFrame>
  );
}
