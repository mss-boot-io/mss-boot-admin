import Footer from '@/components/Footer';
import { getUserRefreshToken, postUserFakeCaptcha, postUserLogin } from '@/services/admin/user';
import {
  GithubOutlined,
  LockOutlined,
  MailOutlined,
  MobileOutlined,
  UserOutlined,
} from '@ant-design/icons';
import {
  LoginForm,
  ProFormCaptcha,
  ProFormCheckbox,
  ProFormInstance,
  ProFormText,
} from '@ant-design/pro-components';
import { useEmotionCss } from '@ant-design/use-emotion-css';
import { FormattedMessage, Helmet, history, Link, SelectLang, useIntl, useModel } from '@umijs/max';
import { message, Tabs } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import { flushSync } from 'react-dom';
import Settings from '../../../../config/defaultSettings';
import { useRequest } from 'ahooks';
import { LarkOutlined } from '@/components/MssBoot/icon';
import { resolveSafeRedirect } from './redirect';
import { getAppConfigsProfile } from '@/services/admin/appConfig';

function randToken(): string {
  let result = '';
  const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  const charactersLength = characters.length;
  for (let i = 0; i < 28; i++) {
    // 可以根据需要调整长度
    result += characters.charAt(Math.floor(Math.random() * charactersLength));
  }
  return result;
}

export type ActionIconsFormProps = {
  fetchUserInfo: () => void;
};

export function persistLoginState(
  data: API.LoginResponse,
  autoLogin?: boolean,
  currentHref = window.location.href,
) {
  if (data.code !== 200 || !data.token) {
    return undefined;
  }

  localStorage.setItem('token', data.token);
  localStorage.setItem('token.expire', data.expire?.toString() || '');
  localStorage.setItem('autoLogin', autoLogin?.toString() || 'false');

  return resolveSafeRedirect(currentHref);
}

const ActionIcons: React.FC<ActionIconsFormProps> = (props) => {
  const { initialState } = useModel('@@initialState');
  const scopes = initialState?.appConfig?.security?.githubScope?.replace(/,/g, '+') || '';

  const langClassName = useEmotionCss(({ token }) => {
    return {
      marginLeft: '8px',
      color: 'rgba(0, 0, 0, 0.2)',
      fontSize: '24px',
      verticalAlign: 'middle',
      cursor: 'pointer',
      transition: 'color 0.3s',
      '&:hover': {
        color: token.colorPrimaryActive,
      },
    };
  });

  return (
    <>
      {initialState?.appConfig?.security?.githubEnabled && (
        <GithubOutlined
          key="GithubOutlined"
          className={langClassName}
          onClick={async () => {
            localStorage.removeItem('login.type');
            localStorage.removeItem('github.token');
            localStorage.removeItem('token');
            localStorage.removeItem('github.state');
            const state = 'ghs_' + randToken();
            localStorage.setItem('github.state', state);
            const loginURL = `https://github.com/login/oauth/authorize?client_id=${initialState?.appConfig?.security?.githubClientId}&response_type=code&scope=${scopes}&state=${state}`;
            const w = window.open('about:blank');
            // @ts-ignore
            w.location.href = loginURL;
            const intervalId = setInterval(() => {
              const loginType = localStorage.getItem('login.type');
              const token = localStorage.getItem('token');
              if (token && loginType === 'github') {
                clearInterval(intervalId);
                try {
                  props.fetchUserInfo();
                } catch (e) {
                  message.error('登录失败，请重试！');
                  return;
                } finally {
                  //登录成功跳转
                  const redirect = resolveSafeRedirect();
                  setTimeout(() => {
                    history.push(redirect);
                  }, 1000);
                }
              }
            }, 1000);
          }}
        />
      )}
      {initialState?.appConfig?.security?.larkEnabled && (
        <LarkOutlined
          key="LarkOutlined"
          onClick={async () => {
            localStorage.removeItem('login.type');
            localStorage.removeItem('lark.token');
            localStorage.removeItem('token');
            localStorage.removeItem('lark.state');
            const state = 'lark' + randToken();
            localStorage.setItem('lark.state', state);
            const loginURL = `https://open.larksuite.com/open-apis/authen/v1/index?redirect_uri=${initialState?.appConfig?.security?.larkRedirectURI}&app_id=${initialState?.appConfig?.security?.larkAppId}&state=${state}`;
            const w = window.open('about:blank');
            // @ts-ignore
            w.location.href = loginURL;
          }}
        />
      )}
    </>
  );
};

const Lang = () => {
  const langClassName = useEmotionCss(({ token }) => {
    return {
      width: 42,
      height: 42,
      lineHeight: '42px',
      position: 'fixed',
      right: 16,
      borderRadius: token.borderRadius,
      ':hover': {
        backgroundColor: token.colorBgTextHover,
      },
      'path.fill': '#555',
    };
  });

  return (
    <div className={langClassName} data-lang>
      {SelectLang && <SelectLang />}
    </div>
  );
};

const Login: React.FC = () => {
  const intl = useIntl();
  const [type, setType] = useState<string>('account');
  const { initialState, setInitialState } = useModel('@@initialState');
  const formRef = useRef<ProFormInstance>();
  const containerClassName = useEmotionCss(() => {
    return {
      display: 'flex',
      flexDirection: 'column',
      height: '100vh',
      overflow: 'auto',
      backgroundImage:
        "url('https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/V-_oS6r-i7wAAAAAAAAAAAAAFl94AQBr')",
      backgroundSize: '100% 100%',
    };
  });

  useEffect(() => {
    if (initialState?.appConfig) {
      return;
    }
    let disposed = false;
    getAppConfigsProfile({ skipErrorHandler: true })
      .then((appConfig) => {
        if (disposed || !appConfig) {
          return;
        }
        setInitialState((state) => ({
          ...state,
          appConfig,
        }));
      })
      .catch(() => {});
    return () => {
      disposed = true;
    };
  }, [initialState?.appConfig, setInitialState]);

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
      const redirect = persistLoginState(data, autoLogin);
      if (redirect) {
        history.push(redirect);
      }
      return;
    }
  };

  useRequest(async () => {
    if (
      localStorage.getItem('autoLogin') &&
      localStorage.getItem('token') &&
      localStorage.getItem('token.expire')
    ) {
      const res = await getUserRefreshToken();
      await loginSuccessed(res, true);
    }
  });

  const handleSubmit = async (values: API.UserLogin, autoLogin?: boolean) => {
    try {
      // 登录
      // @ts-ignore
      const msg = await postUserLogin({ ...values, type });
      await loginSuccessed(msg, autoLogin, true);
      // 如果失败去设置用户错误信息
      // setUserLoginState(msg);
    } catch (error) {
      const defaultLoginFailureMessage = intl.formatMessage({
        id: 'pages.login.failure',
        defaultMessage: '登录失败，请重试！',
      });
      message.error(defaultLoginFailureMessage);
    }
  };
  // const { status, type: loginType } = userLoginState;
  const loginItem = () => {
    let items = [
      {
        key: 'account',
        label: intl.formatMessage({
          id: 'pages.login.accountLogin.tab',
          defaultMessage: '账户密码登录',
        }),
      },
    ];
    const phoneEnabled = initialState?.appConfig?.security?.phoneEnabled;
    const emailEnabled = initialState?.appConfig?.security?.emailEnabled;
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    phoneEnabled &&
      items.push({
        key: 'mobile',
        label: intl.formatMessage({
          id: 'pages.login.phoneLogin.tab',
          defaultMessage: '手机号登录',
        }),
      });
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    emailEnabled &&
      items.push({
        key: 'email',
        label: intl.formatMessage({
          id: 'pages.login.emailLogin.tab',
          defaultMessage: '邮箱登录',
        }),
      });

    return items;
  };

  useEffect(() => {
    const handleVisibilityChange = () => {
      if (!document.hidden) {
        const loginType = localStorage.getItem('login.type');
        const token = localStorage.getItem('token');
        if (token && (loginType === 'github' || loginType === 'lark')) {
          try {
            fetchUserInfo();
          } catch (e) {
            message.error('登录失败，请重试！');
            return;
          } finally {
            //登录成功跳转
            const redirect = resolveSafeRedirect();
            setTimeout(() => {
              history.push(redirect);
            }, 1000);
          }
        }
      }
    };

    // 添加事件监听器
    document.addEventListener('visibilitychange', handleVisibilityChange);

    // 清理事件监听器
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, []);

  return (
    <div className={containerClassName}>
      <Helmet>
        <title>
          {intl.formatMessage({
            id: 'menu.login',
            defaultMessage: '登录页',
          })}
          - {Settings.title}
        </title>
      </Helmet>
      <Lang />
      <div
        style={{
          flex: '1',
          padding: '32px 0',
        }}
      >
        <LoginForm
          contentStyle={{
            minWidth: 280,
            maxWidth: '75vw',
          }}
          formRef={formRef}
          logo={
            <img
              alt="logo"
              src={
                initialState?.appConfig?.base?.websiteLogo ||
                'https://docs.mss-boot-io.top/favicon.ico'
              }
            />
          }
          title={initialState?.appConfig?.base?.websiteName || 'mss-boot-io'}
          subTitle={
            initialState?.appConfig?.base?.websiteDescription ||
            intl.formatMessage({ id: 'pages.login.form.title' })
          }
          initialValues={{
            autoLogin: true,
          }}
          actions={[
            initialState?.appConfig?.security?.githubEnabled ||
            initialState?.appConfig?.security?.larkEnabled ? (
              <FormattedMessage
                key="loginWith"
                id="pages.login.loginWith"
                defaultMessage="其他登录方式"
              />
            ) : (
              ''
            ),
            <ActionIcons fetchUserInfo={fetchUserInfo} key="icons" />,
            initialState?.appConfig?.security?.registerEnabled ? (
              <p>
                还没有账号? &nbsp;
                <span
                  role="link"
                  tabIndex={0}
                  style={{ color: '#1677ff', cursor: 'pointer' }}
                  onClick={() => history.push('/user/register')}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      history.push('/user/register');
                    }
                  }}
                >
                  <FormattedMessage id="pages.login.signup" defaultMessage="注册账户" />
                </span>
              </p>
            ) : (
              ''
            ),
          ]}
          onFinish={async (values) => {
            await handleSubmit(values as API.UserLogin, values.autoLogin);
          }}
        >
          <Tabs activeKey={type} onChange={setType} items={loginItem()} />

          {type === 'account' && (
            <>
              <ProFormText
                name="username"
                fieldProps={{
                  size: 'large',
                  prefix: <UserOutlined />,
                }}
                placeholder={intl.formatMessage({
                  id: 'pages.login.username.placeholder',
                })}
                rules={[
                  {
                    required: true,
                    message: (
                      <FormattedMessage
                        id="pages.login.username.required"
                        defaultMessage="请输入用户名!"
                      />
                    ),
                  },
                ]}
              />
              <ProFormText.Password
                name="password"
                fieldProps={{
                  size: 'large',
                  prefix: <LockOutlined />,
                }}
                placeholder={intl.formatMessage({
                  id: 'pages.login.password.placeholder',
                })}
                rules={[
                  {
                    required: true,
                    message: (
                      <FormattedMessage
                        id="pages.login.password.required"
                        defaultMessage="请输入密码！"
                      />
                    ),
                  },
                ]}
              />
            </>
          )}

          {type === 'mobile' && (
            <>
              <ProFormText
                fieldProps={{
                  size: 'large',
                  prefix: <MobileOutlined />,
                }}
                name="mobile"
                placeholder={intl.formatMessage({
                  id: 'pages.login.phoneNumber.placeholder',
                  defaultMessage: '手机号',
                })}
                rules={[
                  {
                    required: true,
                    message: (
                      <FormattedMessage
                        id="pages.login.phoneNumber.required"
                        defaultMessage="请输入手机号！"
                      />
                    ),
                  },
                  {
                    pattern: /^1\d{10}$/,
                    message: (
                      <FormattedMessage
                        id="pages.login.phoneNumber.invalid"
                        defaultMessage="手机号格式错误！"
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
                    id: 'pages.login.phoneLogin.getVerificationCode',
                    defaultMessage: '获取验证码',
                  });
                }}
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
                onGetCaptcha={async (phone) => {
                  const result = await postUserFakeCaptcha({
                    phone,
                  });
                  if (!result) {
                    return;
                  }
                  message.success('获取验证码成功！验证码为：1234');
                }}
              />
            </>
          )}
          {type === 'email' && (
            <>
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
                name="captcha"
                phoneName="email"
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
                    useBy: 'login',
                  });
                  if (!result) {
                    return;
                  }
                  message.success('获取验证码成功！');
                }}
              />
            </>
          )}
          <div
            style={{
              marginBottom: 24,
            }}
          >
            <ProFormCheckbox noStyle name="autoLogin">
              <FormattedMessage id="pages.login.rememberMe" defaultMessage="自动登录" />
            </ProFormCheckbox>

            <Link
              to="/user/forget"
              style={{
                float: 'right',
              }}
            >
              <FormattedMessage id="pages.login.forgotPassword" defaultMessage="忘记密码" />
            </Link>
          </div>
        </LoginForm>
      </div>
      <Footer />
    </div>
  );
};

export default Login;
