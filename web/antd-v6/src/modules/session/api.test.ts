import { describe, expect, it, vi } from 'vitest';
import { createSessionAPI } from './api';

const record = {
  id: 'session/1',
  userID: 'user/1',
  username: 'alice',
  loginAt: '2026-08-15T00:00:00Z',
  lastSeenAt: '2026-08-15T01:00:00Z',
  expiredAt: '2026-08-15T02:00:00Z',
  revoked: false,
};

describe('online session API', () => {
  it('uses protected relative endpoints and encodes path identifiers', async () => {
    const client = vi.fn(async (path: string) => {
      if (path.includes('/user/')) return { affected: 1, userID: 'user/1' };
      return record;
    });
    const api = createSessionAPI(client);

    await api.loadOne('session/1');
    await api.revokeOne('session/1');
    await expect(api.revokeUser('user/1')).resolves.toEqual({ affected: 1, userID: 'user/1' });

    expect(client).toHaveBeenNthCalledWith(1, '/online-sessions/session%2F1', {
      method: 'GET',
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(2, '/online-sessions/session%2F1', {
      method: 'DELETE',
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(3, '/online-sessions/user/user%2F1', {
      method: 'DELETE',
      skipErrorHandler: true,
    });
  });

  it('sends only the bounded list contract', async () => {
    const client = vi.fn(async () => ({ data: [record], total: 1, current: 1, pageSize: 20 }));
    const api = createSessionAPI(client);
    const params = { current: 1, pageSize: 20, status: 'active' } as const;

    await expect(api.loadPage(params)).resolves.toMatchObject({ total: 1 });
    expect(client).toHaveBeenCalledWith('/online-sessions', {
      method: 'GET',
      params,
      skipErrorHandler: true,
    });
  });
});
