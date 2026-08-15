import { history, request, useParams } from '@umijs/max';
import { Result, Spin } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { resolveSafeRedirect } from '@/shared/auth/redirect';
import { assertNoBrowserCredential } from '@/shared/auth/session';

interface OAuthCallbackResponse {
  redirect?: string;
}

export default function OAuthCallbackPage() {
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
    request<OAuthCallbackResponse>(`/user/session/${encodeURIComponent(provider)}/callback`, {
      method: 'POST',
      data: { code, state },
      skipErrorHandler: true,
    })
      .then((result) => {
        assertNoBrowserCredential(result);
        history.replace(resolveSafeRedirect(result.redirect));
      })
      .catch(() => setError(true));
  }, [provider]);

  if (error) {
    return (
      <Result
        status="error"
        title="第三方登录失败"
        subTitle="授权结果无效或已过期，请返回登录页重试。"
      />
    );
  }

  return (
    <div className="grid min-h-screen place-items-center">
      <Spin size="large" tip="正在完成安全登录…" />
    </div>
  );
}
