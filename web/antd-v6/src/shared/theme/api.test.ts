import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getRequestStatus } from '../api/errors';
import {
  createThemeAPI,
  formatThemeETag,
  type ThemeRequestClient,
  ThemeRevisionConflictError,
} from './api';
import type { ThemeScopeResource } from './contract';

const client = vi.fn<ThemeRequestClient>();
const api = createThemeAPI(client);
const userBase: ThemeScopeResource = {
  scope: 'user',
  revision: '7',
  versioned: true,
  overrides: { colorWeak: false },
};

describe('canonical theme transport', () => {
  beforeEach(() => client.mockReset());

  it('loads the public application layer and canonical personal layer', async () => {
    client
      .mockResolvedValueOnce({
        base: { websiteName: 'MSS' },
        theme: { _meta: { v: 1, scope: 'application', revision: '2' } },
      })
      .mockResolvedValueOnce({
        colorWeak: false,
        _meta: { v: 1, scope: 'user', revision: '7' },
      });

    expect((await api.loadApplicationProfile()).theme.revision).toBe('2');
    expect((await api.loadThemeResource('user')).overrides).toEqual({ colorWeak: false });
    expect(client).toHaveBeenNthCalledWith(
      2,
      '/user-configs/theme',
      expect.objectContaining({
        headers: { Accept: 'application/vnd.mss.theme.v1+json' },
      }),
    );
  });

  it('uses the exact strong ETag and returns the canonical mutation response', async () => {
    client.mockResolvedValue({
      layout: 'side',
      _meta: { v: 1, scope: 'user', revision: '8' },
    });
    const result = await api.patchThemeResource('user', { layout: 'side' }, userBase);
    expect(formatThemeETag(userBase)).toBe('"theme-user-7"');
    expect(client).toHaveBeenCalledWith(
      '/user-configs/theme',
      expect.objectContaining({
        method: 'PUT',
        data: { data: { layout: 'side' } },
        headers: expect.objectContaining({ 'If-Match': '"theme-user-7"' }),
      }),
    );
    expect(result.revision).toBe('8');
  });

  it('surfaces the authoritative 412 resource without silently retrying', async () => {
    const failure = Object.assign(new Error('revision conflict'), {
      response: {
        status: 412,
        data: {
          data: {
            current: {
              layout: 'mix',
              _meta: { v: 1, scope: 'user', revision: '8' },
            },
          },
        },
      },
    });
    expect(getRequestStatus(failure)).toBe(412);
    const conflictAPI = createThemeAPI(async () => Promise.reject(failure));
    let caught: unknown;
    try {
      await conflictAPI.patchThemeResource('user', { layout: 'side' }, userBase);
    } catch (error) {
      caught = error;
    }
    expect(caught).toBeInstanceOf(ThemeRevisionConflictError);
    expect(caught).toMatchObject({
      current: { revision: '8', overrides: { layout: 'mix' } },
    });
  });
});
