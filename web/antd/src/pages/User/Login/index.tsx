import Footer from '@/components/Footer';
import { postUserFakeCaptcha, postUserLogin, postUserRefreshToken } from '@/services/admin/user';
import {
  GithubOutlined,
  LockOutlined,
  MailOutlined,
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
import { Alert, message, Tabs } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import { flushSync } from 'react-dom';
import Settings from '../../../../config/defaultSettings';
import { useRequest } from 'ahooks';
import { LarkOutlined } from '@/components/MssBoot/icon';
import { resolveSafeRedirect } from './redirect';
import { OAuthAuthorizationError, openOAuthAuthorization } from '@/utils/oauth';
import {
  clearNonPersistentAuthStorage,
  clearTransientAuthToken,
  setTransientAuthToken,
} from '@/utils/authStorage';
import {
  clearUserThemeProfile,
  type ThemeRuntimeCoordinatorState,
  type ThemeRuntimeState,
} from '@/utils/themeSettings';
import {
  applyAuthenticatedThemeProfiles,
  clearThemeIdentitySession,
  isThemeAuthSessionActive,
  loadAuthenticatedThemeProfiles,
  rotateThemeAuthSession,
  writeAuthenticatedThemeSnapshots,
} from '@/utils/themeSession';
import ThemeRuntimeBridge from '@/components/MssBoot/ThemeRuntimeBridge';
import {
  isEmailChallengeFlowAvailable,
  useEmailChallengeAvailability,
} from './emailChallengeAccess';

export type ActionIconsFormProps = {
  fetchUserInfo: (authSessionId: string) => Promise<unknown>;
};

export type ActivatedLoginSession = {
  authSessionId: string;
  redirect: string;
};

export function persistLoginState(
  data: API.LoginResponse,
  autoLogin?: boolean,
  currentHref = window.location.href,
) {
  if (data.code !== 200 || !data.token) {
    return undefined;
  }

  clearTransientAuthToken();
  const authSessionId = rotateThemeAuthSession({ persistent: true });
  localStorage.setItem('token', data.token);
  localStorage.setItem('token.expire', data.expire?.toString() || '');
  localStorage.setItem('autoLogin', autoLogin?.toString() || 'false');

  return { authSessionId, redirect: resolveSafeRedirect(currentHref) };
}

export function activateOAuthLoginSession(credential: string, currentHref = window.location.href) {
  const authSessionId = rotateThemeAuthSession({ persistent: false });
  setTransientAuthToken(credential);
  return { authSessionId, redirect: resolveSafeRedirect(currentHref) };
}

export function hasAutoLoginSession(storage: Pick<Storage, 'getItem'> = window.localStorage) {
  return (
    storage.getItem('autoLogin') === 'true' &&
    Boolean(storage.getItem('token')) &&
    Boolean(storage.getItem('token.expire'))
  );
}

const ActionIcons: React.FC<ActionIconsFormProps> = (props) => {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState');
  const [oauthProvider, setOAuthProvider] = useState<API.OAuthProvider>();

  const startOAuthLogin = async (provider: API.OAuthProvider) => {
    if (oauthProvider) {
      return;
    }
    setOAuthProvider(provider);
    try {
      const result = await openOAuthAuthorization(provider, 'login');
      if (result.intent !== 'login') {
        throw new Error('OAuth login returned the wrong intent');
      }
      const session = activateOAuthLoginSession(result.token);
      const userInfo = await props.fetchUserInfo(session.authSessionId);
      if (!userInfo) {
        throw new Error('OAuth login could not load the Admin session');
      }
      message.success(intl.formatMessage({ id: 'pages.login.success' }));
      history.push(session.redirect);
    } catch (error) {
      clearTransientAuthToken();
      clearThemeIdentitySession();
      const messageID =
        error instanceof OAuthAuthorizationError &&
        (error.code === 'timeout' || error.code === 'closed')
          ? `pages.login.oauth2.${error.code}`
          : 'pages.login.failure';
      message.error(intl.formatMessage({ id: messageID }));
    } finally {
      setOAuthProvider(undefined);
    }
  };

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
          aria-busy={oauthProvider === 'github'}
          aria-disabled={Boolean(oauthProvider)}
          onClick={() => void startOAuthLogin('github')}
        />
      )}
      {initialState?.appConfig?.security?.larkEnabled && (
        <LarkOutlined
          key="LarkOutlined"
          aria-busy={oauthProvider === 'lark'}
          aria-disabled={Boolean(oauthProvider)}
          onClick={() => void startOAuthLogin('lark')}
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
  const emailChallenge = useEmailChallengeAvailability('login');
  const refreshedProfile = { security: emailChallenge.security } as API.AppConfigProfile;
  const emailLoginAvailable = emailChallenge.available;
  const registrationAvailable =
    !emailChallenge.checking && isEmailChallengeFlowAvailable(refreshedProfile, 'register');
  const activeLoginType = type === 'email' && emailLoginAvailable ? 'email' : 'account';
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
    // getInitialState only runs at bootstrap. Clear a document-scoped OAuth
    // session and any previous user's identity/theme when history remounts the
    // login route in this SPA (including forced 401 redirects).
    clearNonPersistentAuthStorage();
    clearThemeIdentitySession();
    setInitialState(
      (state) =>
        ({
          ...clearUserThemeProfile(state || {}),
          currentUser: undefined,
        } as typeof state),
    );
  }, [setInitialState]);

  const fetchUserInfo = async (authSessionId: string) => {
    const [userInfo, profiles] = await Promise.all([
      initialState?.fetchUserInfo?.(),
      loadAuthenticatedThemeProfiles(),
    ]);
    if (userInfo && isThemeAuthSessionActive(authSessionId)) {
      let reconciledState: ThemeRuntimeState | undefined;
      let authoritativePrevious: ThemeRuntimeCoordinatorState['layers'] | undefined;
      flushSync(() => {
        setInitialState((state) => {
          authoritativePrevious = state?.themeRuntime?.layers;
          const next = applyAuthenticatedThemeProfiles(
            state || {},
            userInfo,
            profiles,
            authSessionId,
          );
          reconciledState = next;
          return next as typeof state;
        });
      });
      if (reconciledState) {
        await writeAuthenticatedThemeSnapshots(reconciledState, authSessionId, {
          authoritativePrevious,
        });
      }
      return isThemeAuthSessionActive(authSessionId) ? userInfo : undefined;
    }
    return undefined;
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
      const session = persistLoginState(data, autoLogin);
      if (!session) return;
      const userInfo = await fetchUserInfo(session.authSessionId);
      if (userInfo && isThemeAuthSessionActive(session.authSessionId)) {
        history.push(session.redirect);
      }
      return;
    }
  };

  useRequest(async () => {
    if (hasAutoLoginSession()) {
      const res = await postUserRefreshToken();
      await loginSuccessed(res, true);
    }
  });

  const handleSubmit = async (values: API.UserLogin, autoLogin?: boolean) => {
    try {
      // 登录
      // @ts-ignore
      const msg = await postUserLogin({ ...values, type: activeLoginType });
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
    // eslint-disable-next-line @typescript-eslint/no-unused-expressions
    emailLoginAvailable &&
      items.push({
        key: 'email',
        label: intl.formatMessage({
          id: 'pages.login.emailLogin.tab',
          defaultMessage: '邮箱登录',
        }),
      });

    return items;
  };

  return (
    <div className={containerClassName}>
      <ThemeRuntimeBridge />
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
            registrationAvailable ? (
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
          <Tabs activeKey={activeLoginType} onChange={setType} items={loginItem()} />

          {!emailChallenge.checking &&
            emailChallenge.security.emailEnabled === true &&
            !emailLoginAvailable && (
              <Alert
                showIcon
                type="warning"
                message={intl.formatMessage({
                  id: 'pages.login.emailChallengeUnavailableDescription',
                  defaultMessage:
                    '邮箱验证服务当前不可用，请稍后重试或使用账号密码登录。',
                })}
                style={{ marginBottom: 16 }}
              />
            )}

          {activeLoginType === 'account' && (
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

          {activeLoginType === 'email' && emailLoginAvailable && (
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

            {emailLoginAvailable && (
              <Link
                to="/user/forget"
                style={{
                  float: 'right',
                }}
              >
                <FormattedMessage id="pages.login.forgotPassword" defaultMessage="忘记密码" />
              </Link>
            )}
          </div>
        </LoginForm>
      </div>
      <Footer />
    </div>
  );
};

export default Login;
