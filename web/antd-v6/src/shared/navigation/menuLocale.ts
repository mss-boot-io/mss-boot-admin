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
  const lastMenuIndex = segments.lastIndexOf('menu');
  if (lastMenuIndex >= 0) return segments.slice(lastMenuIndex).join('.');
  return `menu.${segments.join('.')}`;
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
