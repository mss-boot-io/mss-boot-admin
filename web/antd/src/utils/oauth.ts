import { postUserClearAuthCookie, postUserOauth2Authorize } from '@/services/admin/user';
import { clearAuthStorage } from './authStorage';

const OAUTH_CHANNEL_PREFIX = 'mss-admin-oauth:';
const DEFAULT_OAUTH_TIMEOUT_MS = 5 * 60 * 1000;
const POPUP_CLOSED_POLL_MS = 250;

export const LEGACY_OAUTH_STORAGE_KEYS = [
  'github.token',
  'lark.token',
  'bindingType',
  'github.state',
  'lark.state',
] as const;

export type OAuthAuthorizationErrorCode =
  | 'popup-blocked'
  | 'unsupported'
  | 'timeout'
  | 'closed'
  | 'invalid-response';

export class OAuthAuthorizationError extends Error {
  code: OAuthAuthorizationErrorCode;

  constructor(code: OAuthAuthorizationErrorCode, message: string) {
    super(message);
    this.name = 'OAuthAuthorizationError';
    this.code = code;
  }
}

type OAuthResultBase = {
  attemptID: string;
  code: number;
  provider: API.OAuthProvider;
};

export type OAuthAuthorizationResult =
  | (OAuthResultBase & {
      intent: 'login';
      token: string;
      expire?: string;
    })
  | (OAuthResultBase & {
      intent: 'binding';
    })
  | (OAuthResultBase & {
      intent: 'integration';
      credential: string;
      credentialExpiresAt?: string;
    });

export function getOAuthChannelName(attemptID: string) {
  const normalizedAttemptID = attemptID.trim();
  if (!normalizedAttemptID || normalizedAttemptID.length > 256) {
    throw new OAuthAuthorizationError('invalid-response', 'OAuth attempt ID is invalid');
  }
  return `${OAUTH_CHANNEL_PREFIX}${normalizedAttemptID}`;
}

export function purgeLegacyOAuthStorage(
  storage: Pick<Storage, 'removeItem'> = window.localStorage,
) {
  LEGACY_OAUTH_STORAGE_KEYS.forEach((key) => storage.removeItem(key));
}

export function requireServerOAuthIntent(
  response: Pick<API.OAuthCallbackResponse, 'intent'>,
): API.OAuthIntent {
  if (
    response.intent === 'login' ||
    response.intent === 'binding' ||
    response.intent === 'integration'
  ) {
    return response.intent;
  }
  throw new OAuthAuthorizationError(
    'invalid-response',
    'OAuth callback response has no valid server intent',
  );
}

function hasProviderCredentialFields(value: Record<string, unknown>) {
  return ['accessToken', 'refreshToken', 'tokenType', 'expiry', 'refreshExpiry'].some(
    (key) => key in value,
  );
}

function toSafeOAuthResult(
  value: unknown,
  expected?: {
    attemptID: string;
    provider: API.OAuthProvider;
    intent: API.OAuthIntent;
  },
): OAuthAuthorizationResult {
  if (!value || typeof value !== 'object') {
    throw new OAuthAuthorizationError('invalid-response', 'OAuth callback response is invalid');
  }
  const response = value as Record<string, unknown>;
  if (hasProviderCredentialFields(response)) {
    throw new OAuthAuthorizationError(
      'invalid-response',
      'OAuth callback exposed a provider credential',
    );
  }

  const intent = requireServerOAuthIntent(response as Pick<API.OAuthCallbackResponse, 'intent'>);
  const attemptID = typeof response.attemptID === 'string' ? response.attemptID : '';
  const provider = response.provider;
  const code = response.code;
  if (
    !attemptID ||
    (provider !== 'github' && provider !== 'lark') ||
    typeof code !== 'number' ||
    code !== 200 ||
    (expected &&
      (attemptID !== expected.attemptID ||
        provider !== expected.provider ||
        intent !== expected.intent))
  ) {
    throw new OAuthAuthorizationError('invalid-response', 'OAuth callback response is invalid');
  }

  const safeProvider = provider as API.OAuthProvider;
  const base: OAuthResultBase = { attemptID, code, provider: safeProvider };
  if (intent === 'login') {
    if (typeof response.token !== 'string' || !response.token) {
      throw new OAuthAuthorizationError('invalid-response', 'Admin login token is missing');
    }
    return {
      ...base,
      intent,
      token: response.token,
      ...(typeof response.expire === 'string' ? { expire: response.expire } : {}),
    };
  }
  if (intent === 'integration') {
    if (typeof response.credential !== 'string' || !response.credential) {
      throw new OAuthAuthorizationError('invalid-response', 'OAuth credential handle is missing');
    }
    return {
      ...base,
      intent,
      credential: response.credential,
      ...(typeof response.credentialExpiresAt === 'string'
        ? { credentialExpiresAt: response.credentialExpiresAt }
        : {}),
    };
  }
  return { ...base, intent };
}

export function publishOAuthCallbackResult(response: API.OAuthCallbackResponse) {
  if (typeof BroadcastChannel === 'undefined') {
    throw new OAuthAuthorizationError('unsupported', 'BroadcastChannel is unavailable');
  }
  const safeResult = toSafeOAuthResult(response);
  const channel = new BroadcastChannel(getOAuthChannelName(safeResult.attemptID));
  try {
    channel.postMessage(safeResult);
  } finally {
    channel.close();
  }
  return safeResult;
}

function getAuthorizationTimeout(expiresAt: string) {
  const expiresAtMs = Date.parse(expiresAt);
  if (!Number.isFinite(expiresAtMs)) {
    return DEFAULT_OAUTH_TIMEOUT_MS;
  }
  return Math.max(0, Math.min(expiresAtMs - Date.now(), DEFAULT_OAUTH_TIMEOUT_MS));
}

/**
 * Opens a blank popup while the user gesture is active, then coordinates the
 * attempt through a same-origin channel. The listener is active before the
 * popup navigates, and only the intent-specific safe result is returned.
 */
export async function openOAuthAuthorization(
  provider: API.OAuthProvider,
  intent: API.OAuthIntent,
): Promise<OAuthAuthorizationResult> {
  if (typeof BroadcastChannel === 'undefined') {
    throw new OAuthAuthorizationError('unsupported', 'BroadcastChannel is unavailable');
  }
  if (intent === 'login') {
    // The callback runs in a same-origin popup and shares localStorage with
    // the opener. Remove any prior Admin bearer before both authorize and
    // callback requests so a stale session cannot block a new login attempt.
    clearAuthStorage();
  }
  const popup = window.open('about:blank', '_blank');
  if (!popup) {
    throw new OAuthAuthorizationError('popup-blocked', 'OAuth popup was blocked');
  }
  popup.opener = null;

  let authorization: API.OAuthAuthorizeResponse;
  let channel: BroadcastChannel;
  try {
    if (intent === 'login') {
      // Best-effort cookie expiry keeps a revoked or key-rotated HttpOnly JWT
      // from making OptionalAuth reject both the authorize and callback legs.
      try {
        await postUserClearAuthCookie({ skipAuthToken: true, skipErrorHandler: true });
      } catch {
        // Rolling deployments may briefly route to an older API instance.
      }
    }
    authorization = await postUserOauth2Authorize(
      { provider, intent },
      intent === 'login' ? { skipAuthToken: true } : undefined,
    );
    if (!authorization?.authorizeURL || !authorization.attemptID) {
      throw new OAuthAuthorizationError(
        'invalid-response',
        'OAuth authorization response is incomplete',
      );
    }
    channel = new BroadcastChannel(getOAuthChannelName(authorization.attemptID));
  } catch (error) {
    popup.close();
    throw error;
  }

  return new Promise<OAuthAuthorizationResult>((resolve, reject) => {
    let timeoutID: ReturnType<typeof setTimeout> | undefined;
    let closePollID: ReturnType<typeof setInterval> | undefined;
    let settled = false;
    let handleMessage: (event: MessageEvent<unknown>) => void = () => undefined;

    const cleanup = (closePopup: boolean) => {
      if (timeoutID) {
        clearTimeout(timeoutID);
      }
      if (closePollID) {
        clearInterval(closePollID);
      }
      channel.removeEventListener('message', handleMessage);
      channel.close();
      if (closePopup && !popup.closed) {
        try {
          popup.close();
        } catch {
          // The popup may already be cross-origin or closing.
        }
      }
    };

    const fail = (error: unknown, closePopup = true) => {
      if (settled) {
        return;
      }
      settled = true;
      cleanup(closePopup);
      reject(error);
    };

    const succeed = (result: OAuthAuthorizationResult) => {
      if (settled) {
        return;
      }
      settled = true;
      cleanup(true);
      resolve(result);
    };

    handleMessage = (event: MessageEvent<unknown>) => {
      try {
        succeed(
          toSafeOAuthResult(event.data, {
            attemptID: authorization.attemptID,
            provider,
            intent,
          }),
        );
      } catch (error) {
        fail(error);
      }
    };

    channel.addEventListener('message', handleMessage);

    try {
      popup.location.href = authorization.authorizeURL;
    } catch (error) {
      fail(error);
      return;
    }

    timeoutID = setTimeout(() => {
      fail(new OAuthAuthorizationError('timeout', 'OAuth authorization timed out'));
    }, getAuthorizationTimeout(authorization.expiresAt));
    closePollID = setInterval(() => {
      if (popup.closed) {
        fail(new OAuthAuthorizationError('closed', 'OAuth popup closed before completion'), false);
      }
    }, POPUP_CLOSED_POLL_MS);
  });
}
