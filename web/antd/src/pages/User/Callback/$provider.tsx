import { getUserProviderCallback, postUserLogin } from '@/services/admin/user';
import { useIntl, useParams, useSearchParams } from '@umijs/max';
import { Alert, Flex, Spin, message } from 'antd';
import React, { useEffect } from 'react';

const Index: React.FC = () => {
  const intl = useIntl();
  const params = useParams();
  // @ts-ignore
  const provider: API.LoginProvider = params.provider || 'github';
  const statePrefix = provider === 'github' ? 'ghs_' : provider;
  const [searchParams] = useSearchParams();
  const [load, setLoad] = React.useState(true);
  const code = searchParams.get('code');
  const state = searchParams.get('state');

  useEffect(() => {
    if (state) {
      if (
        !localStorage.getItem(`${provider}.state`) ||
        localStorage.getItem(`${provider}.state`) !== state
      ) {
        message.error('state error');
        return;
      }

      getUserProviderCallback({ provider: provider, code: code!, state: state }).then(
        (res: API.OauthToken) => {
          // getGithubCallback(req).then((res: API.OauthToken) => {
          if (res && res.accessToken) {
            localStorage.setItem(`${provider}.token`, res.accessToken);
            setLoad(false);
            message.success(intl.formatMessage({ id: 'pages.login.oauth2.success' }));

            //get token
            if (state.startsWith(statePrefix) && !localStorage.getItem('token')) {
              postUserLogin({ password: res.accessToken, type: provider }).then((msg) => {
                if (msg.code === 200 && msg.token) {
                  message.success(intl.formatMessage({ id: 'pages.login.success' }));
                  //set token to localstorage
                  localStorage.setItem('token', msg.token);
                  localStorage.setItem('login.type', provider);

                  setTimeout(() => {
                    window.close();
                  }, 2000);
                }
              });
            } else {
              setTimeout(() => {
                window.close();
              }, 2000);
            }
          }
        },
      );
    }
  }, [code, state]);

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
          message={intl.formatMessage({ id: 'pages.login.callback.success.title' })}
          description={intl.formatMessage({ id: 'pages.login.callback.success.description' })}
          type="info"
        />
      )}
    </Flex>
  );
};

export default Index;
