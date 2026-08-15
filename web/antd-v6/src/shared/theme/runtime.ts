import {
  CODE_THEME_DEFAULTS,
  compareThemeRevisions,
  type ResolvedTheme,
  resolveTheme,
  THEME_SETTING_KEYS,
  type ThemeScope,
  type ThemeScopeResource,
} from './contract';

export interface ThemeRuntimeSnapshot {
  application?: ThemeScopeResource;
  user?: ThemeScopeResource;
  resolved: ResolvedTheme;
  degradedScopes: ThemeScope[];
}

export type ThemeReconcileStatus = 'applied' | 'duplicate' | 'stale' | 'same-revision-conflict';

const listeners = new Set<() => void>();

function sameOverrides(left: ThemeScopeResource, right: ThemeScopeResource): boolean {
  return THEME_SETTING_KEYS.every(
    (key) =>
      Object.hasOwn(left.overrides, key) === Object.hasOwn(right.overrides, key) &&
      left.overrides[key] === right.overrides[key],
  );
}

function buildSnapshot(
  application?: ThemeScopeResource,
  user?: ThemeScopeResource,
  degradedScopes: ThemeScope[] = [],
): ThemeRuntimeSnapshot {
  return {
    application,
    user,
    resolved: resolveTheme(application, user, CODE_THEME_DEFAULTS),
    degradedScopes: [...new Set(degradedScopes)],
  };
}

let snapshot = buildSnapshot();

function publish(next: ThemeRuntimeSnapshot) {
  snapshot = next;
  for (const listener of listeners) listener();
}

export function getThemeRuntimeSnapshot(): ThemeRuntimeSnapshot {
  return snapshot;
}

export function subscribeThemeRuntime(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function replaceThemeRuntime(input: {
  application?: ThemeScopeResource;
  user?: ThemeScopeResource;
  degradedScopes?: ThemeScope[];
}): ThemeRuntimeSnapshot {
  const next = buildSnapshot(input.application, input.user, input.degradedScopes);
  publish(next);
  return next;
}

export function reconcileThemeResource(
  resource: ThemeScopeResource,
  options: { authoritative?: boolean } = {},
): ThemeReconcileStatus {
  const current = snapshot[resource.scope];
  if (current) {
    const order = compareThemeRevisions(resource.revision, current.revision);
    if (order < 0 && !options.authoritative) return 'stale';
    if (order === 0 && sameOverrides(resource, current)) {
      if (!options.authoritative) return 'duplicate';
    } else if (order === 0 && !options.authoritative) {
      return 'same-revision-conflict';
    }
  }

  const application = resource.scope === 'application' ? resource : snapshot.application;
  const user = resource.scope === 'user' ? resource : snapshot.user;
  publish(
    buildSnapshot(
      application,
      user,
      snapshot.degradedScopes.filter((scope) => scope !== resource.scope),
    ),
  );
  return 'applied';
}

export function clearUserThemeRuntime(): ThemeRuntimeSnapshot {
  const next = buildSnapshot(
    snapshot.application,
    undefined,
    snapshot.degradedScopes.filter((scope) => scope !== 'user'),
  );
  publish(next);
  return next;
}
