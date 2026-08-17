import type { ProLayoutProps } from '@ant-design/pro-components';
import { addLocale, history } from '@umijs/max';
import { languageAPI } from '../../modules/language/api';
import { registerSupportedLanguageProfile } from '../../modules/language/runtime';
import { getRequestStatus } from '../api/client';
import { fetchAuthorizedMenu } from '../auth/authorization';
import { fetchCurrentUser, isPublicPath, redirectToLogin } from '../auth/session';
import type { InitialState } from '../auth/types';
import { queryClient, queryKeys } from '../query/client';
import { loadApplicationProfile, loadThemeResource } from '../theme/api';
import { type ApplicationProfile, buildLayoutSettings } from '../theme/contract';
import { getThemeRuntimeSnapshot, replaceThemeRuntime } from '../theme/runtime';

interface Settled<T> {
  data?: T;
  error?: unknown;
}

async function settle<T>(promise: Promise<T>): Promise<Settled<T>> {
  try {
    return { data: await promise };
  } catch (error) {
    return { error };
  }
}

async function settleWithin<T>(promise: Promise<T>, timeoutMs: number): Promise<Settled<T>> {
  let timeout: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      settle(promise),
      new Promise<Settled<T>>((resolve) => {
        timeout = setTimeout(
          () => resolve({ error: new Error('Optional startup resource timed out') }),
          timeoutMs,
        );
      }),
    ]);
  } finally {
    if (timeout) clearTimeout(timeout);
  }
}

function applyLanguageProfile(
  profile: Settled<Awaited<ReturnType<typeof languageAPI.loadProfile>>>,
): void {
  if (profile.data) registerSupportedLanguageProfile(profile.data, addLocale);
}

async function loadVerifiedCurrentUser() {
  const value = await queryClient.fetchQuery({
    queryKey: queryKeys.currentUser,
    queryFn: async () => (await fetchCurrentUser()) ?? null,
    staleTime: 0,
  });
  return value ?? undefined;
}

function currentLayoutSettings(profile?: ApplicationProfile): Partial<ProLayoutProps> {
  return buildLayoutSettings(getThemeRuntimeSnapshot().resolved.settings, profile?.base);
}

export async function loadInitialState(): Promise<InitialState> {
  const {
    clearThemeIdentitySession,
    ensureThemeAuthSession,
    readThemeBootstrapSnapshot,
    readThemeSnapshot,
    writeThemeSnapshot,
  } = await import('../theme/snapshot');
  const publicRoute = isPublicPath(history.location.pathname);
  const bootstrap = readThemeBootstrapSnapshot();
  replaceThemeRuntime({
    application: bootstrap.application,
    degradedScopes: bootstrap.application ? ['application'] : [],
  });
  const applicationProfilePromise = settle(
    queryClient.fetchQuery({
      queryKey: queryKeys.applicationProfile,
      queryFn: loadApplicationProfile,
    }),
  );
  const languageProfilePromise = settleWithin(
    queryClient.fetchQuery({
      queryKey: queryKeys.languageProfile,
      queryFn: languageAPI.loadProfile,
      staleTime: 5 * 60_000,
    }),
    2_500,
  );

  if (publicRoute) {
    const [application, languageProfile] = await Promise.all([
      applicationProfilePromise,
      languageProfilePromise,
    ]);
    applyLanguageProfile(languageProfile);
    const applicationTheme = application.data?.theme ?? bootstrap.application;
    replaceThemeRuntime({
      application: applicationTheme,
      degradedScopes: application.error ? ['application'] : [],
    });
    if (application.data?.theme) {
      void writeThemeSnapshot(application.data.theme, undefined, Date.now(), {
        authoritativePrevious: bootstrap.application,
      });
    }
    return {
      applicationProfile: application.data,
      authorizationVersion: 0,
      authorizedMenu: [],
      fetchCurrentUser: loadVerifiedCurrentUser,
      settings: currentLayoutSettings(application.data),
      themeDegradedScopes: application.error ? ['application'] : undefined,
    };
  }

  const [application, identity, languageProfile] = await Promise.all([
    applicationProfilePromise,
    settle(loadVerifiedCurrentUser()),
    languageProfilePromise,
  ]);
  applyLanguageProfile(languageProfile);
  const applicationTheme = application.data?.theme ?? bootstrap.application;
  replaceThemeRuntime({
    application: applicationTheme,
    degradedScopes: application.error ? ['application'] : [],
  });
  if (application.data?.theme) {
    void writeThemeSnapshot(application.data.theme, undefined, Date.now(), {
      authoritativePrevious: bootstrap.application,
    });
  }

  if (identity.error) {
    return {
      applicationProfile: application.data,
      authorizationVersion: 0,
      authorizedMenu: [],
      fetchCurrentUser: loadVerifiedCurrentUser,
      settings: currentLayoutSettings(application.data),
      startupFailure: { area: 'identity', status: getRequestStatus(identity.error) },
      themeDegradedScopes: application.error ? ['application'] : undefined,
    };
  }

  const currentUser = identity.data;
  if (!currentUser) {
    clearThemeIdentitySession();
    redirectToLogin();
    return {
      applicationProfile: application.data,
      authorizationVersion: 0,
      authorizedMenu: [],
      fetchCurrentUser: loadVerifiedCurrentUser,
      settings: currentLayoutSettings(application.data),
      themeDegradedScopes: application.error ? ['application'] : undefined,
    };
  }

  const authSessionId = ensureThemeAuthSession(currentUser.id);
  const userBootstrap = readThemeSnapshot('user', authSessionId);
  replaceThemeRuntime({
    application: applicationTheme,
    user: userBootstrap,
    degradedScopes: [
      ...(application.error ? (['application'] as const) : []),
      ...(userBootstrap ? (['user'] as const) : []),
    ],
  });

  const [authorizedMenu, userTheme] = await Promise.all([
    settle(
      queryClient.fetchQuery({
        queryKey: queryKeys.authorizedMenu(currentUser.id),
        queryFn: () => fetchAuthorizedMenu(currentUser),
        staleTime: 0,
      }),
    ),
    settle(
      queryClient.fetchQuery({
        queryKey: queryKeys.theme('user', currentUser.id),
        queryFn: () => loadThemeResource('user'),
      }),
    ),
  ]);

  const degradedScopes = [
    ...(application.error ? (['application'] as const) : []),
    ...(userTheme.error ? (['user'] as const) : []),
  ];
  const userThemeResource = userTheme.data ?? userBootstrap;
  replaceThemeRuntime({
    application: applicationTheme,
    user: userThemeResource,
    degradedScopes: [...degradedScopes],
  });
  if (userTheme.data) {
    void writeThemeSnapshot(userTheme.data, authSessionId, Date.now(), {
      authoritativePrevious: userBootstrap,
    });
  }

  return {
    applicationProfile: application.data,
    authorizationVersion: 1,
    currentUser,
    authSessionId,
    authorizedMenu: authorizedMenu.data ?? [],
    fetchCurrentUser: loadVerifiedCurrentUser,
    settings: currentLayoutSettings(application.data),
    startupFailure: authorizedMenu.error
      ? { area: 'authorization', status: getRequestStatus(authorizedMenu.error) }
      : undefined,
    themeDegradedScopes: degradedScopes.length > 0 ? [...degradedScopes] : undefined,
  };
}
