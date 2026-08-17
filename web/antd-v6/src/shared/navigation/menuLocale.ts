interface MenuIntl {
  formatMessage(descriptor: { id: string; defaultMessage?: string }): string;
}

/**
 * Normalize both canonical menu IDs and migration-era values where CompleteName
 * prefixed an already-complete child ID. The last `menu` segment is authoritative.
 */
export function resolveMenuLocaleID(value: string | undefined): string {
  const trimmed = value?.trim() ?? '';
  if (!trimmed) return '';

  const segments = trimmed.split('.').filter(Boolean);
  if (!segments.length) return '';

  // The authorization endpoint uses the historical leaf name `menu` for
  // /menu, while the V6 route and locale catalog use `menu-management` to
  // avoid colliding with the `menu.` locale namespace itself.
  if (segments.length === 1 && segments[0] === 'menu') return 'menu.menu-management';
  if (segments[0] !== 'menu') return `menu.${segments.join('.')}`;

  // Migration-era CompleteName values can repeat the complete canonical ID.
  // Only a later `menu` followed by the original root segment starts such a
  // repeated ID; a final business leaf such as `menu.authority.menu` does not.
  const rootSegment = segments[1];
  let canonicalStart = 0;
  for (let index = 2; index < segments.length - 1; index += 1) {
    if (segments[index] === 'menu' && segments[index + 1] === rootSegment) {
      canonicalStart = index;
    }
  }
  return segments.slice(canonicalStart).join('.');
}

/** ProLayout prefixes route names with `menu.` when locale support is enabled. */
export function resolveLayoutMenuName(value: string | undefined): string | undefined {
  const localeID = resolveMenuLocaleID(value);
  return localeID ? localeID.slice('menu.'.length) : undefined;
}

export function formatMenuLabel(intl: MenuIntl, value: string | undefined): string {
  const localeID = resolveMenuLocaleID(value);
  if (!localeID) return '';
  return intl.formatMessage({ id: localeID, defaultMessage: value?.trim() || localeID });
}
