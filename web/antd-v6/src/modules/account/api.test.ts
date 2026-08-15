import { describe, expect, it, vi } from 'vitest';
import { createAccountAPI } from './api';

describe('account API', () => {
  it('uses self-service mutation methods and encodes token identifiers', async () => {
    const client = vi
      .fn()
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({
        id: 'pat-1',
        userID: 'user-1',
        expiredAt: '2027-01-01T00:00:00Z',
        revoked: false,
        token: 'one-time',
      })
      .mockResolvedValueOnce({});
    const api = createAccountAPI(client);

    await api.updateProfile({ name: 'New name' });
    await expect(api.rotateAccessToken('unsafe/id')).resolves.toMatchObject({ token: 'one-time' });
    await api.revokeAccessToken('unsafe/id');

    expect(client).toHaveBeenNthCalledWith(1, '/user/userInfo', {
      method: 'PUT',
      data: { name: 'New name' },
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(2, '/user-auth-token/unsafe%2Fid/refresh', {
      method: 'PUT',
      skipErrorHandler: true,
    });
    expect(client).toHaveBeenNthCalledWith(3, '/user-auth-token/unsafe%2Fid/revoke', {
      method: 'PUT',
      skipErrorHandler: true,
    });
  });

  it('starts only the server-owned browser-session binding flow', async () => {
    const client = vi.fn().mockResolvedValue({
      authorizeURL: 'https://github.com/login/oauth/authorize?state=opaque',
      attemptID: 'attempt-1',
      expiresAt: '2026-08-15T03:00:00Z',
    });
    const api = createAccountAPI(client);

    await expect(api.startOAuthAuthorization('github', 'binding')).resolves.toMatchObject({
      attemptID: 'attempt-1',
    });
    expect(client).toHaveBeenCalledWith('/user/session/oauth2/authorize', {
      method: 'POST',
      data: { provider: 'github', intent: 'binding' },
      skipErrorHandler: true,
    });
  });

  it('persists one notification key through the authenticated user scope', async () => {
    const client = vi.fn().mockResolvedValue({});
    await createAccountAPI(client).updateNotificationSetting('email', true);
    expect(client).toHaveBeenCalledWith('/user-configs/notification', {
      method: 'PUT',
      data: { data: { email: true } },
      skipErrorHandler: true,
    });
  });
});
