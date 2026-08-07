import { postUserProviderCallback } from '@/services/admin/user';
import { publishOAuthCallbackResult, requireServerOAuthIntent } from '@/utils/oauth';
import { useIntl, useParams } from '@umijs/max';
import { Alert, Flex, Spin, message } from 'antd';
import React, { useEffect, useState } from 'react';

let consumedCallback:
  | {
      cleanURL: string;
      params: { code: string | null; state: string | null };
    }
  | undefined;

let callbackExecution: Promise<void> | undefined;

export function consumeOAuthCallbackParams() {
  const callbackURL = new URL(window.location.href);
  const code = callbackURL.searchParams.get('code');
  const state = callbackURL.searchParams.get('state');
  const currentURL = `${callbackURL.pathname}${callbackURL.search}${callbackURL.hash}`;
  if (!code && !state && consumedCallback?.cleanURL === currentURL) {
    return consumedCallback.params;
  }
  callbackURL.searchParams.delete('code');
  callbackURL.searchParams.delete('state');
  const cleanURL = `${callbackURL.pathname}${callbackURL.search}${callbackURL.hash}`;
  window.history.replaceState(window.history.state, '', cleanURL);
  const callbackParams = { code, state };
  consumedCallback = { cleanURL, params: callbackParams };
  return callbackParams;
}

function completeOAuthCallback(provider: API.OAuthProvider, code: string, state: string) {
  if (callbackExecution) {
    return callbackExecution;
  }
  callbackExecution = postUserProviderCallback(
    { provider },
    { code, state },
    { skipErrorHandler: true },
  )
    .then((response) => {
      requireServerOAuthIntent(response);
      publishOAuthCallbackResult(response);
    })
    .catch(() => {
      throw new Error('OAuth callback failed');
    });
  return callbackExecution;
}

const Index: React.FC = () => {
  const intl = useIntl();
  const params = useParams();
  const provider =
    params.provider === 'github' || params.provider === 'lark' ? params.provider : undefined;
  const [{ code, state }] = useState(consumeOAuthCallbackParams);
  const [load, setLoad] = useState(true);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let closeTimer: ReturnType<typeof setTimeout> | undefined;
    let disposed = false;

    if (!provider || !state || !code) {
      setLoad(false);
      setFailed(true);
      message.error(intl.formatMessage({ id: 'pages.login.callback.parameters' }));
      return undefined;
    }

    // Callback failures stay local to this popup so a binding failure cannot
    // clear the still-valid Admin session in the opener.
    completeOAuthCallback(provider, code, state)
      .then(() => {
        if (disposed) {
          return;
        }
        setLoad(false);
        message.success(intl.formatMessage({ id: 'pages.login.oauth2.success' }));
        closeTimer = setTimeout(() => window.close(), 1000);
      })
      .catch(() => {
        if (disposed) {
          return;
        }
        setLoad(false);
        setFailed(true);
        message.error(intl.formatMessage({ id: 'pages.login.callback.failure' }));
      });

    return () => {
      disposed = true;
      if (closeTimer) {
        clearTimeout(closeTimer);
      }
    };
  }, [code, intl, provider, state]);

  return (
    <Flex gap="middle" vertical>
      {load && (
        <Spin
          aria-label={intl.formatMessage({ id: 'pages.login.callback.loading' })}
          fullscreen
          size="large"
        />
      )}
      {!load && (
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
