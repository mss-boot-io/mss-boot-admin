import { describe, expect, it, vi } from 'vitest';
import { createLanguageAPI, type LanguageRequestOptions } from './api';

const detail = {
  id: 'language/1',
  name: 'en-US',
  remark: '',
  status: 'enabled',
  updatedAt: '2026-08-15T00:00:00Z',
  defines: [{ id: 'welcome', group: 'menu', key: 'welcome', value: 'Welcome' }],
};

describe('language API', () => {
  it('uses bounded summary queries and encoded detail paths', async () => {
    const client = vi.fn(async (path: string) =>
      path === '/languages' ? { data: [detail], total: 1, current: 1, pageSize: 20 } : detail,
    );
    const api = createLanguageAPI(client);
    const params = { current: 1, pageSize: 20, status: 'all', name: 'en' } as const;

    await api.loadPage(params);
    await api.loadOne('language/1');

    expect(client).toHaveBeenNthCalledWith(1, '/languages', {
      method: 'GET',
      params: { current: 1, pageSize: 20, status: undefined, name: 'en', view: 'summary' },
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(2, '/languages/language%2F1', {
      method: 'GET',
      skipErrorHandler: true,
    });
  });

  it('loads the public profile through the relative transport', async () => {
    const client = vi.fn(async () => ({ 'en-US': { 'menu.language': 'Languages' } }));
    const api = createLanguageAPI(client);

    await expect(api.loadProfile()).resolves.toEqual({
      'en-US': { 'menu.language': 'Languages' },
    });
    expect(client).toHaveBeenCalledWith('/language/profile', {
      method: 'GET',
      skipErrorHandler: true,
    });
  });

  it('never sends client-owned language identity and includes update revision', async () => {
    const client = vi.fn(async (_path: string, _options: LanguageRequestOptions) => detail);
    const api = createLanguageAPI(client);
    const values = {
      name: 'en-US',
      status: 'enabled' as const,
      defines: [{ group: 'menu', key: 'welcome', value: 'Welcome' }],
    };
    const clientValues = { ...values, id: 'client-owned' };

    await api.create(clientValues);
    await api.update('language/1', values, detail.updatedAt);
    await api.remove('language/1');

    expect(client.mock.calls[0]?.[1]).toMatchObject({
      method: 'POST',
      data: { name: 'en-US', status: 'enabled', remark: '', defines: values.defines },
    });
    expect(client.mock.calls[0]?.[1]?.data).not.toHaveProperty('id');
    expect(client.mock.calls[1]?.[1]).toMatchObject({
      method: 'PUT',
      data: { expectedUpdatedAt: detail.updatedAt },
    });
    expect(client).toHaveBeenNthCalledWith(3, '/languages/language%2F1', {
      method: 'DELETE',
      skipErrorHandler: true,
    });
  });
});
