import { postUserOauth2Authorize } from '@/services/admin/user';

/**
 * Open a popup synchronously so browser popup blockers allow it, then navigate
 * only to the provider URL issued by the server. State and browser binding are
 * owned by the backend; callers never generate or persist OAuth state.
 */
export async function openOAuthAuthorization(
  provider: API.LoginProvider,
  intent: API.OAuthIntent,
) {
  const popup = window.open('about:blank', '_blank');
  if (!popup) {
    throw new Error('OAuth popup was blocked');
  }
  popup.opener = null;
  try {
    const authorization = await postUserOauth2Authorize({ provider, intent });
    if (!authorization?.authorizeURL) {
      throw new Error('OAuth authorization URL is missing');
    }
    popup.location.href = authorization.authorizeURL;
    return popup;
  } catch (error) {
    popup.close();
    throw error;
  }
}

export function requireServerOAuthIntent(token: Pick<API.OauthToken, 'intent'>): API.OAuthIntent {
  if (token.intent === 'login' || token.intent === 'binding' || token.intent === 'integration') {
    return token.intent;
  }
  throw new Error('OAuth callback response has no valid server intent');
}
