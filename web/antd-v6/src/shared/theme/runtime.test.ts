import { beforeEach, describe, expect, it } from 'vitest';
import { parseThemeScopeResource } from './contract';
import {
  clearUserThemeRuntime,
  getThemeRuntimeSnapshot,
  reconcileThemeResource,
  replaceThemeRuntime,
} from './runtime';

describe('theme runtime store', () => {
  beforeEach(() => replaceThemeRuntime({}));

  it('recomputes immediately and removes the personal layer on logout', () => {
    replaceThemeRuntime({
      application: parseThemeScopeResource(
        { layout: 'top', _meta: { v: 1, scope: 'application', revision: '2' } },
        'application',
      ),
      user: parseThemeScopeResource(
        { layout: 'side', _meta: { v: 1, scope: 'user', revision: '4' } },
        'user',
      ),
    });
    expect(getThemeRuntimeSnapshot().resolved.settings.layout).toBe('side');
    clearUserThemeRuntime();
    expect(getThemeRuntimeSnapshot().resolved.settings.layout).toBe('top');
  });

  it('rejects older and divergent same-revision events', () => {
    const current = parseThemeScopeResource(
      { colorWeak: false, _meta: { v: 1, scope: 'user', revision: '8' } },
      'user',
    );
    replaceThemeRuntime({ user: current });
    expect(
      reconcileThemeResource(
        parseThemeScopeResource(
          { colorWeak: true, _meta: { v: 1, scope: 'user', revision: '7' } },
          'user',
        ),
      ),
    ).toBe('stale');
    expect(
      reconcileThemeResource(
        parseThemeScopeResource(
          { colorWeak: true, _meta: { v: 1, scope: 'user', revision: '8' } },
          'user',
        ),
      ),
    ).toBe('same-revision-conflict');
    expect(getThemeRuntimeSnapshot().resolved.settings.colorWeak).toBe(false);
  });

  it('does not let an older authoritative response roll back healthy runtime state', () => {
    replaceThemeRuntime({
      application: parseThemeScopeResource(
        { layout: 'top', _meta: { v: 1, scope: 'application', revision: '10' } },
        'application',
      ),
    });

    expect(
      reconcileThemeResource(
        parseThemeScopeResource(
          { layout: 'side', _meta: { v: 1, scope: 'application', revision: '9' } },
          'application',
        ),
        { authoritative: true },
      ),
    ).toBe('stale');
    expect(getThemeRuntimeSnapshot().resolved.settings.layout).toBe('top');
  });

  it('lets an authoritative response replace an explicitly degraded bootstrap hint', () => {
    replaceThemeRuntime({
      application: parseThemeScopeResource(
        { layout: 'top', _meta: { v: 1, scope: 'application', revision: '10' } },
        'application',
      ),
      degradedScopes: ['application'],
    });

    expect(
      reconcileThemeResource(
        parseThemeScopeResource(
          { layout: 'side', _meta: { v: 1, scope: 'application', revision: '9' } },
          'application',
        ),
        { authoritative: true },
      ),
    ).toBe('applied');
    expect(getThemeRuntimeSnapshot().resolved.settings.layout).toBe('side');
    expect(getThemeRuntimeSnapshot().degradedScopes).toEqual([]);
  });
});
