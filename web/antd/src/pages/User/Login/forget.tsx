import React, { useRef } from 'react';
import {
  ProFormCaptcha,
  ProFormInstance,
  ProCard,
  ProFormText,
  StepsForm,
} from '@ant-design/pro-components';
import { message } from 'antd';
import { useIntl, FormattedMessage, history } from '@umijs/max';
import Footer from '@/components/Footer';
import { LockOutlined, MailOutlined } from '@ant-design/icons';
import { postUserFakeCaptcha, postUserResetPassword } from '@/services/admin/user';
import { fieldIntl } from '@/util/fieldIntl';
import AuthShell from '@/components/AuthShell';

const Forget: React.FC = () => {
  const intl = useIntl();
  const formRef = useRef<ProFormInstance>();

  return (
    <AuthShell titleDefaultMessage="忘记密码">
      <ProCard>
        <StepsForm<API.ResetPasswordRequest>
          formRef={formRef}
          onFinish={async (values) => {
            await postUserResetPassword(values);
            message
              .success(
                intl.formatMessage({
                  id: 'pages.login.passwordResetSuccess',
                  defaultMessage: '密码重置成功',
                }),
              )
              .then(() => history.push('/user/login'));
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
                  useBy: 'resetPassword',
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
              id: 'pages.title.password.reset',
            })}
            stepProps={{
              description: intl.formatMessage({
                id: 'pages.description.password.reset',
                defaultMessage: '这里重置您的密码',
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

export default Forget;
