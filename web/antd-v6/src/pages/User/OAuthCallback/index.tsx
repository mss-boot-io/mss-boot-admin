import { history, request, useIntl, useParams } from '@umijs/max';
import { Result, Spin } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { parseOAuthCallbackOutcome } from '@/modules/account/contracts';
import { resolveSafeRedirect } from '@/shared/auth/redirect';
import { assertNoBrowserCredential, fetchCurrentUser } from '@/shared/auth/session';
import { queryClient, queryKeys } from '@/shared/query/client';
import { rotateThemeAuthSession } from '@/shared/theme/snapshot';

export default function OAuthCallbackPage() {
  const intl = useIntl();
  const { provider } = useParams<{ provider: string }>();
  const callbackStarted = useRef(false);
  const [error, setError] = useState(false);

  useEffect(() => {
    // React development StrictMode replays effects. The OAuth state is
    // deliberately single-use, so never submit the same provider callback
    // twice from one mounted page.
    if (callbackStarted.current) return;
    callbackStarted.current = true;
    const params = new URLSearchParams(history.location.search);
    const code = params.get('code');
    const state = params.get('state');
    history.replace(history.location.pathname);
    if (!provider || !code || !state) {
      setError(true);
      return;
    }
    request<unknown>(`/user/session/${encodeURIComponent(provider)}/callback`, {
      method: 'POST',
      data: { code, state },
      skipErrorHandler: true,
    })
      .then(async (result) => {
        assertNoBrowserCredential(result);
        const outcome = parseOAuthCallbackOutcome(result, provider);
        const currentUser = await fetchCurrentUser();
        if (!currentUser) throw new Error('OAuth session identity was not established');
        if (outcome.intent === 'binding') {
          await queryClient.invalidateQueries({
            queryKey: queryKeys.accountOAuth(currentUser.id),
          });
          window.location.replace('/account/settings?tab=connections&binding=success');
          return;
        }
        if (outcome.intent === 'reauthentication') {
          await queryClient.invalidateQueries({
            queryKey: queryKeys.accountSecurity(currentUser.id),
          });
          window.location.replace('/account/settings?tab=security&reauthentication=success');
          return;
        }
        rotateThemeAuthSession(currentUser.id);
        window.location.replace(resolveSafeRedirect(undefined));
      })
      .catch(() => setError(true));
  }, [provider]);

  if (error) {
    return (
      <Result
        status="error"
        title={intl.formatMessage({ id: 'pages.oauth.failureTitle' })}
        subTitle={intl.formatMessage({ id: 'pages.oauth.failureDescription' })}
      />
    );
  }

  return (
    <div className="grid min-h-screen place-items-center">
      <Spin size="large" tip={intl.formatMessage({ id: 'pages.oauth.completing' })} />
    </div>
  );
}
