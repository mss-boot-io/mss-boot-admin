import { describe, expect, it, vi } from 'vitest';
import { createAdministrationAPI } from './api';

describe('administration API', () => {
  it('maps hierarchy pagination to the legacy-compatible backend query', async () => {
    const client = vi.fn(async () => ({ data: [], total: 0, current: 2, pageSize: 20 }));
    const api = createAdministrationAPI(client);

    await api.departments.list({ current: 2, pageSize: 20, name: 'Platform', status: 'enabled' });

    expect(client).toHaveBeenCalledWith('/departments', {
      method: 'GET',
      params: {
        name: 'Platform',
        status: 'enabled',
        page: 2,
        pageSize: 20,
        parentID: '',
      },
      skipErrorHandler: true,
    });
  });

  it('uses a strong revision precondition for role authorization writes', async () => {
    const client = vi.fn(async () => ({ roleID: 'role-1', revision: '8', paths: ['/users'] }));
    const api = createAdministrationAPI(client);

    const next = await api.roles.saveAuthorization(
      { roleID: 'role-1', revision: '7', paths: [], etag: 'ignored-client-metadata' },
      ['/users', '/users'],
    );

    expect(client).toHaveBeenCalledWith('/role/authorize/role-1', {
      method: 'POST',
      data: { paths: ['/users'] },
      headers: { 'If-Match': '"role-authorization-role-1-7"' },
      skipErrorHandler: true,
    });
    expect(next.revision).toBe('8');
  });

  it('omits the password from generic user updates', async () => {
    const client = vi.fn(async (_path: string, options: { data?: unknown }) => ({
      id: 'user-1',
      username: (options.data as { username: string }).username,
      email: 'operator@example.com',
      status: 'enabled',
    }));
    const api = createAdministrationAPI(client);

    await api.users.update('user-1', {
      username: 'operator_1',
      email: 'operator@example.com',
      password: 'NeverPersistThis123',
      status: 'enabled',
    });

    expect(client.mock.calls[0]?.[1]?.data).not.toHaveProperty('password');
  });
});
