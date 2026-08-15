import { describe, expect, it, vi } from 'vitest';
import { createOperationsAPI, runtimeLogExportPath } from './api';

const timestamp = '2026-08-16T06:00:00Z';

describe('operations API', () => {
  it('maps task pagination and never sends server-owned state', async () => {
    const client = vi.fn(async (_path: string, options: { data?: unknown }) => ({
      id: 'task-1',
      createdAt: timestamp,
      updatedAt: timestamp,
      name: (options.data as { name: string }).name,
      provider: 'default',
      spec: '0 * * * * *',
      status: 'disabled',
      remark: '',
      command: '[]',
      args: '[]',
      protocol: 'https',
      endpoint: 'example.test/health',
      method: 'GET',
      timeout: 30,
    }));
    const api = createOperationsAPI(client);

    await api.tasks.create({
      name: ' Health ',
      provider: 'default',
      spec: '0 * * * * *',
      protocol: 'https',
      endpoint: 'example.test/health',
      method: 'GET',
    });

    expect(client.mock.calls[0]?.[1]?.data).toEqual({
      name: 'Health',
      provider: 'default',
      spec: '0 * * * * *',
      remark: '',
      timeout: 30,
      protocol: 'https',
      endpoint: 'example.test/health',
      method: 'GET',
      body: '',
      metadata: '',
    });
    expect(client.mock.calls[0]?.[1]?.data).not.toHaveProperty('status');
  });

  it('uses owner-scoped notice routes and the runtime page parameter', async () => {
    const client = vi.fn(async (path: string) => {
      if (path === '/notices') return { data: [], total: 0, current: 1, pageSize: 20 };
      return { list: [], total: 0, truncated: false };
    });
    const api = createOperationsAPI(client);

    await api.notices.list({ current: 1, pageSize: 20, type: 'message', status: 'all' });
    await api.logs.runtime({ page: 2, pageSize: 50, level: 'error', keyword: 'literal.*' });

    expect(client).toHaveBeenNthCalledWith(1, '/notices', {
      method: 'GET',
      params: {
        current: 1,
        pageSize: 20,
        title: undefined,
        status: undefined,
        type: 'message',
      },
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(2, '/logs', {
      method: 'GET',
      params: {
        page: 2,
        pageSize: 50,
        level: 'error',
        keyword: 'literal.*',
        startTime: undefined,
        endTime: undefined,
      },
      skipErrorHandler: true,
    });
  });

  it('encodes export filters without accepting a filesystem path', () => {
    expect(
      runtimeLogExportPath({
        level: 'error',
        keyword: 'token & failure',
        startTime: '2026-08-01T00:00:00Z',
        endTime: '2026-08-02T00:00:00Z',
      }),
    ).toBe(
      '/admin/api/logs/export?level=error&keyword=token+%26+failure&startTime=2026-08-01T00%3A00%3A00Z&endTime=2026-08-02T00%3A00%3A00Z',
    );
  });
});
