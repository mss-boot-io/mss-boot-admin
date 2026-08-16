import { parseThemeScopeResource, resolveTheme, type ThemeScopeResource } from './contract';
import { APPLICATION_THEME_SNAPSHOT_KEY, THEME_SNAPSHOT_TTL_MS } from './storage';

interface FirstPaintEnvelope {
  v?: unknown;
  expiresAt?: unknown;
  resource?: unknown;
}

/** Apply only the non-sensitive public application hint before React mounts. */
export function applyThemeFirstPaintHint(now = Date.now()) {
  if (typeof document === 'undefined') return;
  let application: ThemeScopeResource | undefined;
  try {
    const raw = window.localStorage.getItem(APPLICATION_THEME_SNAPSHOT_KEY);
    if (raw) {
      const envelope = JSON.parse(raw) as FirstPaintEnvelope;
      if (
        envelope.v === 1 &&
        typeof envelope.expiresAt === 'number' &&
        Number.isFinite(envelope.expiresAt) &&
        envelope.expiresAt > now &&
        envelope.expiresAt <= now + THEME_SNAPSHOT_TTL_MS
      ) {
        const parsed = parseThemeScopeResource(envelope.resource, 'application');
        if (parsed.versioned) application = parsed;
      }
    }
  } catch {
    // First paint is an optional hint; startup performs the authoritative read.
  }
  const settings = resolveTheme(application).settings;
  const root = document.documentElement;
  root.dataset.mssTheme = settings.navTheme;
  root.style.colorScheme = settings.navTheme === 'realDark' ? 'dark' : 'light';
  root.style.setProperty('--mss-theme-color-primary', settings.colorPrimary);
}
