import { QueryClient } from '@tanstack/react-query';
import { beforeEach, describe, expect, it } from 'vitest';
import { queryKeys } from '@/shared/query/client';
import { parseApplicationProfile, parseThemeScopeResource } from '@/shared/theme/contract';
import { getThemeRuntimeSnapshot, replaceThemeRuntime } from '@/shared/theme/runtime';
import { applyCanonicalThemeResource } from './apply';

describe('canonical theme application', () => {
  beforeEach(() => replaceThemeRuntime({}));

  it('updates the shared runtime and matching React Query resources', () => {
    const client = new QueryClient();
    client.setQueryData(
      queryKeys.applicationProfile,
      parseApplicationProfile({
        base: { websiteName: 'MSS' },
        theme: { _meta: { v: 1, scope: 'application', revision: '1' } },
      }),
    );
    const resource = parseThemeScopeResource(
      { layout: 'top', _meta: { v: 1, scope: 'application', revision: '2' } },
      'application',
    );

    const applied = applyCanonicalThemeResource(client, resource);

    expect(applied.status).toBe('applied');
    expect(applied.settings.layout).toBe('top');
    expect(client.getQueryData(queryKeys.theme('application'))).toEqual(resource);
    expect(client.getQueryData(queryKeys.applicationProfile)).toMatchObject({ theme: resource });
  });

  it('keeps a newer cross-tab revision when an older request completes later', () => {
    const client = new QueryClient();
    const newer = parseThemeScopeResource(
      { colorWeak: true, _meta: { v: 1, scope: 'user', revision: '12' } },
      'user',
    );
    replaceThemeRuntime({ user: newer });
    const older = parseThemeScopeResource(
      { colorWeak: false, _meta: { v: 1, scope: 'user', revision: '11' } },
      'user',
    );

    const applied = applyCanonicalThemeResource(client, older, 'user-1');

    expect(applied.status).toBe('stale');
    expect(applied.resource).toEqual(newer);
    expect(getThemeRuntimeSnapshot().resolved.settings.colorWeak).toBe(true);
    expect(client.getQueryData(queryKeys.theme('user', 'user-1'))).toEqual(newer);
  });
});
