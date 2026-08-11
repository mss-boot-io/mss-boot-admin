import AuthShell from '@/components/AuthShell';
import Footer from '@/components/Footer';
import { getAppConfigsProfile } from '@/services/admin/appConfig';
import { reconcileThemeProfile } from '@/utils/themeSettings';
import { FormattedMessage, Link, useModel } from '@umijs/max';
import React, { useEffect, useState } from 'react';

export type EmailChallengeFlow = 'login' | 'register' | 'resetPassword';

const emailChallengeProfileTimeoutMs = 4000;

const loadEmailChallengeProfile = () => {
  let fallbackTimer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_, reject) => {
    fallbackTimer = setTimeout(
      () => reject(new Error('email challenge profile request timed out')),
      emailChallengeProfileTimeoutMs,
    );
  });

  return Promise.race([
    getAppConfigsProfile({
      skipErrorHandler: true,
      timeout: emailChallengeProfileTimeoutMs,
    }),
    timeout,
  ]).finally(() => {
    if (fallbackTimer !== undefined) clearTimeout(fallbackTimer);
  });
};

const failClosedSecurity = (
  security: API.AppConfigSecurityProfile | undefined,
): API.AppConfigSecurityProfile => ({
  ...(security || {}),
  emailChallengeReady: false,
});

export const isEmailChallengeFlowAvailable = (
  profile: API.AppConfigProfile | undefined,
  flow: EmailChallengeFlow,
) => {
  const security = profile?.security;
  if (security?.emailEnabled !== true || security.emailChallengeReady !== true) {
    return false;
  }
  return flow !== 'register' || security.registerEnabled === true;
};

export const useEmailChallengeAvailability = (flow: EmailChallengeFlow) => {
  const { initialState, setInitialState } = useModel('@@initialState');
  const initialSecurity = initialState?.appConfig?.security as
    | API.AppConfigSecurityProfile
    | undefined;
  const [snapshot, setSnapshot] = useState<{
    checking: boolean;
    security: API.AppConfigSecurityProfile;
  }>({
    checking: true,
    security: failClosedSecurity(initialSecurity),
  });

  useEffect(() => {
    let disposed = false;
    const publish = (
      security: API.AppConfigSecurityProfile,
      profile?: API.AppConfigProfile,
    ) => {
      if (disposed) return;
      setSnapshot({ checking: false, security });
      setInitialState((state) => {
        if (profile?.theme !== undefined) {
          return reconcileThemeProfile(
            state || {},
            { ...profile, security },
            'application',
            { allowLegacyReplace: true, authoritative: true },
          ).state as typeof state;
        }
        return {
          ...state,
          appConfig: {
            ...(state?.appConfig || {}),
            ...(profile || {}),
            security,
          },
        };
      });
    };

    loadEmailChallengeProfile()
      .then((profile) => {
        const security = profile?.security;
        publish(
          security && typeof security === 'object'
            ? security
            : failClosedSecurity(initialSecurity),
          profile,
        );
      })
      .catch(() => publish(failClosedSecurity(initialSecurity)));

    return () => {
      disposed = true;
    };
    // Readiness is a live dependency projection. Re-run when this route mounts,
    // not when a cached initial-state object happens to change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [setInitialState]);

  const profile = { security: snapshot.security } as API.AppConfigProfile;
  return {
    checking: snapshot.checking,
    available: !snapshot.checking && isEmailChallengeFlowAvailable(profile, flow),
    security: snapshot.security,
  };
};

const statusStyle: React.CSSProperties = {
  margin: '48px auto',
  maxWidth: 520,
  padding: 24,
  textAlign: 'center',
};

export const EmailChallengeRoute: React.FC<{
  flow: Exclude<EmailChallengeFlow, 'login'>;
  children: React.ReactNode;
}> = ({ flow, children }) => {
  const availability = useEmailChallengeAvailability(flow);
  if (availability.checking) {
    return (
      <AuthShell titleDefaultMessage="邮箱验证">
        <div role="status" style={statusStyle}>
          <FormattedMessage
            id="pages.login.emailChallengeChecking"
            defaultMessage="正在检查邮箱验证服务…"
          />
        </div>
        <Footer />
      </AuthShell>
    );
  }
  if (!availability.available) {
    const registrationDisabled =
      flow === 'register' && availability.security.registerEnabled !== true;
    return (
      <AuthShell titleDefaultMessage="邮箱验证暂不可用">
        <div role="alert" style={statusStyle}>
          <h2>
            <FormattedMessage
              id="pages.login.emailChallengeUnavailable"
              defaultMessage="邮箱验证暂不可用"
            />
          </h2>
          <p>
            {registrationDisabled ? (
              <FormattedMessage
                id="pages.login.registrationUnavailableDescription"
                defaultMessage="当前未开放用户注册。"
              />
            ) : (
              <FormattedMessage
                id="pages.login.emailChallengeUnavailableDescription"
                defaultMessage="邮箱验证服务当前不可用，请稍后重试或使用账号密码登录。"
              />
            )}
          </p>
          <Link to="/user/login">
            <FormattedMessage id="pages.login.backToLogin" defaultMessage="返回登录" />
          </Link>
        </div>
        <Footer />
      </AuthShell>
    );
  }
  return <>{children}</>;
};
