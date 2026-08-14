export function resolveSafeRedirect(
  value: string | null | undefined,
  fallback = '/workplace',
  origin = window.location.origin,
): string {
  if (!value) return fallback;
  try {
    const applicationOrigin = new URL(origin).origin;
    const target = new URL(value, `${applicationOrigin}/`);
    if (target.origin !== applicationOrigin) return fallback;
    return `${target.pathname}${target.search}${target.hash}` || fallback;
  } catch {
    return fallback;
  }
}
