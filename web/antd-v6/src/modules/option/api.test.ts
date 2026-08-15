import { describe, expect, it, vi } from 'vitest';
import { createOptionAPI, type OptionRequestOptions } from './api';
import type { OptionDetail } from './contract';

const detail: OptionDetail = {
  id: 'option/1',
  category: 'system',
  displayName: 'Status',
  description: '',
  name: 'status',
  remark: '',
  status: 'enabled',
  version: 2,
  builtIn: false,
  updatedAt: '2026-08-15T00:00:00Z',
  items: [{ id: 'item-1', key: 'yes', label: 'Yes', value: 'yes', color: '', sort: 1 }],
};

describe('option API', () => {
  it('uses bounded summary queries and encoded detail paths', async () => {
    const client = vi.fn(async (path: string) =>
      path === '/options' ? { data: [detail], total: 1, current: 1, pageSize: 20 } : detail,
    );
    const api = createOptionAPI(client);
    const params = {
      current: 1,
      pageSize: 20,
      status: 'all',
      category: 'system',
      name: 'stat',
    } as const;
    await api.loadPage(params);
    await api.loadOne('option/1');
    expect(client).toHaveBeenNthCalledWith(1, '/options', {
      method: 'GET',
      params: {
        current: 1,
        pageSize: 20,
        status: undefined,
        category: 'system',
        name: 'stat',
        view: 'summary',
      },
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(2, '/options/option%2F1', {
      method: 'GET',
      skipErrorHandler: true,
    });
  });

  it('sends no client identity on create and exact strong preconditions on mutations', async () => {
    const client = vi.fn(async (_path: string, _options: OptionRequestOptions) => detail);
    const api = createOptionAPI(client);
    const values = {
      category: 'system',
      displayName: 'Status',
      name: 'status',
      status: 'enabled' as const,
      items: [{ key: 'yes', label: 'Yes', value: 'yes', sort: 1 }],
    };
    const clientValues = { ...values, id: 'client-owned' };
    await api.create(clientValues);
    await api.update('option/1', values, detail);
    await api.remove(detail);

    expect(client.mock.calls[0]?.[1]).toMatchObject({ method: 'POST' });
    expect(client.mock.calls[0]?.[1]?.data).not.toHaveProperty('id');
    expect(client.mock.calls[0]?.[1]?.data).not.toHaveProperty('version');
    expect(client.mock.calls[1]?.[1]).toMatchObject({
      method: 'PUT',
      data: { expectedVersion: 2 },
      headers: { 'If-Match': '"option-option/1-v2"' },
    });
    expect(client).toHaveBeenNthCalledWith(3, '/options/option%2F1', {
      method: 'DELETE',
      headers: { 'If-Match': '"option-option/1-v2"' },
      skipErrorHandler: true,
    });
  });
});
