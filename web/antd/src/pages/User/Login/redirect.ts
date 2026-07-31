const DEFAULT_REDIRECT = '/';

export function resolveSafeRedirect(
  currentHref = window.location.href,
  fallback = DEFAULT_REDIRECT,
) {
  let currentUrl: URL;

  try {
    currentUrl = new URL(currentHref, window.location.origin);
  } catch (e) {
    return fallback;
  }

  const redirect = currentUrl.searchParams.get('redirect');
  if (!redirect) {
    return fallback;
  }

  try {
    const redirectUrl = new URL(redirect, currentUrl.origin);
    if (redirectUrl.origin !== currentUrl.origin) {
      return fallback;
    }

    return `${redirectUrl.pathname}${redirectUrl.search}${redirectUrl.hash}` || fallback;
  } catch (e) {
    return fallback;
  }
}
