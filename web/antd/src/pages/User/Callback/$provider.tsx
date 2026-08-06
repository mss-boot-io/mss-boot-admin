import { getUserProviderCallback, postUserLogin } from '@/services/admin/user';
import { useIntl, useParams, useSearchParams } from '@umijs/max';
import { Alert, Flex, Spin, message } from 'antd';
import React, { useEffect } from 'react';
import { requireServerOAuthIntent } from '@/utils/oauth';

const Index: React.FC = () => {
  const intl = useIntl();
  const params = useParams();
  // @ts-ignore
  const provider: API.LoginProvider = params.provider || 'github';
  const [searchParams] = useSearchParams();
  const [load, setLoad] = React.useState(true);
  const [failed, setFailed] = React.useState(false);
  const code = searchParams.get('code');
  const state = searchParams.get('state');

  useEffect(() => {
    if (!state || !code) {
      setLoad(false);
      setFailed(true);
      message.error(intl.formatMessage({ id: 'pages.login.callback.parameters' }));
      return;
    }

    // OAuth state failures are local to this popup. In binding
    // flows a 401 must not trigger the global handler and clear the still-valid
    // primary Admin session from shared localStorage.
    getUserProviderCallback({ provider, code, state }, { skipErrorHandler: true })
      .then(async (res: API.OauthToken) => {
        if (!res?.accessToken) {
          throw new Error('OAuth callback response is invalid');
        }
        const intent = requireServerOAuthIntent(res);
        localStorage.setItem(`${provider}.token`, res.accessToken);
        setLoad(false);
        message.success(intl.formatMessage({ id: 'pages.login.oauth2.success' }));

        if (intent === 'login') {
          const msg = await postUserLogin({ password: res.accessToken, type: provider });
          if (msg.code !== 200 || !msg.token) {
            throw new Error('OAuth login failed');
          }
          message.success(intl.formatMessage({ id: 'pages.login.success' }));
          localStorage.setItem('token', msg.token);
          localStorage.setItem('token.expire', msg.expire?.toString() || '');
          localStorage.setItem('autoLogin', 'false');
          localStorage.setItem('login.type', provider);
        } else if (intent === 'binding') {
          localStorage.setItem('bindingType', provider);
        }

        setTimeout(() => {
          window.close();
        }, 2000);
      })
      .catch(() => {
        setLoad(false);
        setFailed(true);
        message.error(intl.formatMessage({ id: 'pages.login.callback.failure' }));
      });
  }, [code, intl, provider, state]);

  return (
    <Flex gap="middle" vertical>
      <Spin
        tip={intl.formatMessage({ id: 'pages.login.callback.loading' })}
        fullscreen={load}
        size="large"
      />
      {load ? (
        ''
      ) : (
        <Alert
          message={intl.formatMessage({
            id: failed ? 'pages.login.callback.failure' : 'pages.login.callback.success.title',
          })}
          description={intl.formatMessage({
            id: failed ? 'pages.login.callback.retry' : 'pages.login.callback.success.description',
          })}
          type={failed ? 'error' : 'info'}
        />
      )}
    </Flex>
  );
};

export default Index;
