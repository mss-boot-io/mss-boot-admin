import React, { useRef } from 'react';
import {
  ProFormCaptcha,
  ProFormInstance,
  ProCard,
  ProFormText,
  StepsForm,
} from '@ant-design/pro-components';
import { message } from 'antd';
import { useIntl, FormattedMessage, history, useModel } from '@umijs/max';
import Footer from '@/components/Footer';
import { LockOutlined, MailOutlined } from '@ant-design/icons';
import { postUserFakeCaptcha, postUserLogin } from '@/services/admin/user';
import { fieldIntl } from '@/util/fieldIntl';
import { flushSync } from 'react-dom';
import AuthShell from '@/components/AuthShell';
import { resolveSafeRedirect } from './redirect';

const Register: React.FC = () => {
  const intl = useIntl();
  const { initialState, setInitialState } = useModel('@@initialState');
  const formRef = useRef<ProFormInstance>();

  const fetchUserInfo = async () => {
    const userInfo = await initialState?.fetchUserInfo?.();
    if (userInfo) {
      flushSync(() => {
        setInitialState((s) => ({
          ...s,
          currentUser: userInfo,
        }));
      });
    }
  };

  const loginSuccessed = async (data: API.LoginResponse, autoLogin?: boolean, popup?: boolean) => {
    if (data.code === 200 && data.token) {
      if (popup) {
        const defaultLoginSuccessMessage = intl.formatMessage({
          id: 'pages.login.success',
          defaultMessage: '登录成功！',
        });
        message.success(defaultLoginSuccessMessage);
      }
      //set token to localstorage
      localStorage.setItem('token', data.token);
      localStorage.setItem('token.expire', data.expire?.toString() || '');
      localStorage.setItem('autoLogin', autoLogin?.toString() || 'false');
      await fetchUserInfo();
      const redirect = resolveSafeRedirect();
      if (redirect === '/user/login') {
        history.push('/');
        return;
      }
      history.push(redirect);
      return;
    }
  };

  return (
    <AuthShell titleDefaultMessage="注册">
      <ProCard>
        <StepsForm<API.UserLogin>
          formRef={formRef}
          onFinish={async (values) => {
            const msg = await postUserLogin({ ...values, type: 'email_register' });
            await loginSuccessed(msg, true, true);
            message
              .success(
                intl.formatMessage({
                  id: 'pages.login.registerSuccess',
                  defaultMessage: '注册成功',
                }),
              )
              .then(() => history.push('/'));
          }}
          formProps={{
            validateMessages: {
              required: intl.formatMessage({
                id: 'pages.validate.required',
                defaultMessage: '此项为必填',
              }),
            },
          }}
        >
          <StepsForm.StepForm<{
            name: string;
          }>
            name="verify"
            title={intl.formatMessage({
              id: 'pages.title.verifyEmail',
              defaultMessage: '验证邮箱',
            })}
            stepProps={{
              description: intl.formatMessage({
                id: 'pages.description.verifyEmail',
                defaultMessage: '请输入您的邮箱地址',
              }),
            }}
          >
            <ProFormText
              fieldProps={{
                size: 'large',
                prefix: <MailOutlined />,
              }}
              name="email"
              placeholder={intl.formatMessage({
                id: 'pages.login.email.placeholder',
                defaultMessage: '邮箱',
              })}
              rules={[
                {
                  required: true,
                  message: (
                    <FormattedMessage
                      id="pages.login.email.required"
                      defaultMessage="请输入邮箱！"
                    />
                  ),
                },
                {
                  pattern: /^(\w-*\.*)+@(\w-?)+(\.\w{2,})+$/,
                  message: (
                    <FormattedMessage
                      id="pages.login.email.invalid"
                      defaultMessage="邮箱格式错误！"
                    />
                  ),
                },
              ]}
            />
            <ProFormCaptcha
              fieldProps={{
                size: 'large',
                prefix: <LockOutlined />,
              }}
              captchaProps={{
                size: 'large',
              }}
              placeholder={intl.formatMessage({
                id: 'pages.login.captcha.placeholder',
                defaultMessage: '请输入验证码',
              })}
              captchaTextRender={(timing, count) => {
                if (timing) {
                  return `${count} ${intl.formatMessage({
                    id: 'pages.getCaptchaSecondText',
                    defaultMessage: '获取验证码',
                  })}`;
                }
                return intl.formatMessage({
                  id: 'pages.login.emailLogin.getVerificationCode',
                  defaultMessage: '获取验证码',
                });
              }}
              phoneName="email"
              name="captcha"
              rules={[
                {
                  required: true,
                  message: (
                    <FormattedMessage
                      id="pages.login.captcha.required"
                      defaultMessage="请输入验证码！"
                    />
                  ),
                },
              ]}
              onGetCaptcha={async () => {
                // @ts-ignore
                const email: string = formRef.current?.getFieldFormatValue('email');
                const result = await postUserFakeCaptcha({
                  email,
                  useBy: 'register',
                });
                if (!result) {
                  return;
                }
                message.success(
                  intl.formatMessage({
                    id: 'pages.message.getCaptchaSuccess',
                    defaultMessage: '获取验证码成功！',
                  }),
                );
              }}
            />
          </StepsForm.StepForm>
          <StepsForm.StepForm<{
            checkbox: string;
          }>
            name="reset"
            title={intl.formatMessage({
              id: 'pages.title.register',
              defaultMessage: '注册',
            })}
            stepProps={{
              description: intl.formatMessage({
                id: 'pages.description.password.setting',
                defaultMessage: '这里设置您的密码',
              }),
            }}
          >
            <ProFormText.Password
              name="password"
              label={fieldIntl(intl, 'password')}
              placeholder={intl.formatMessage({
                id: 'pages.login.password.placeholder',
              })}
              rules={[
                { required: true },
                { min: 8, message: intl.formatMessage({ id: 'pages.validate.password.min8' }) },
                { max: 20, message: intl.formatMessage({ id: 'pages.validate.password.max20' }) },
                {
                  pattern: /[a-zA-Z]/,
                  message: intl.formatMessage({ id: 'pages.validate.password.letter' }),
                },
                {
                  pattern: /[0-9]/,
                  message: intl.formatMessage({ id: 'pages.validate.password.number' }),
                },
              ]}
            />
            <ProFormText.Password
              name="confirm"
              label={fieldIntl(intl, 'confirm')}
              placeholder={intl.formatMessage({
                id: 'pages.login.password.placeholder',
              })}
              rules={[
                { required: true },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    if (!value || getFieldValue('password') === value) {
                      return Promise.resolve();
                    }
                    return Promise.reject(
                      new Error(
                        intl.formatMessage({
                          id: 'pages.login.passwordInconsistent',
                          defaultMessage: '两次密码不一致',
                        }),
                      ),
                    );
                  },
                }),
              ]}
            />
          </StepsForm.StepForm>
        </StepsForm>
      </ProCard>
      <Footer />
    </AuthShell>
  );
};

export default Register;
