import { request } from '@umijs/max';
import { postTaskOperateId } from './task';
import {
  getUserProviderCallback,
  postUserOauth2Authorize,
  postUserRefreshToken,
} from './user';
import { postUserAuthTokenGenerate } from './userAuthToken';

jest.mock('@umijs/max', () => ({
  request: jest.fn(),
}));

const mockRequest = request as jest.Mock;

describe('security-sensitive mutation clients', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockRequest.mockResolvedValue({});
  });

  it('operates tasks with POST', async () => {
    await postTaskOperateId({ id: 'task-1', operate: 'start' });

    expect(mockRequest).toHaveBeenCalledWith('/admin/api/tasks/task-1/actions/start', {
      method: 'POST',
      params: {},
    });
  });

  it('generates personal access tokens with POST', async () => {
    await postUserAuthTokenGenerate({ validityPeriod: '24h' });

    expect(mockRequest).toHaveBeenCalledWith('/admin/api/user-auth-tokens', {
      method: 'POST',
      params: { validityPeriod: '24h' },
    });
  });

  it('refreshes the login token with POST', async () => {
    await postUserRefreshToken();

    expect(mockRequest).toHaveBeenCalledWith('/admin/api/user/refresh-token', {
      method: 'POST',
    });
  });

  it('starts OAuth with a server-owned POST state flow', async () => {
    await postUserOauth2Authorize({ provider: 'github', intent: 'login' });

    expect(mockRequest).toHaveBeenCalledWith('/admin/api/user/oauth2/authorize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      data: { provider: 'github', intent: 'login' },
    });
  });

  it('keeps OAuth callback failures out of global session cleanup', async () => {
    await getUserProviderCallback(
      { provider: 'github', code: 'code', state: 'state' },
      { skipErrorHandler: true },
    );

    expect(mockRequest).toHaveBeenCalledWith('/admin/api/user/github/callback', {
      method: 'GET',
      params: { code: 'code', state: 'state' },
      skipErrorHandler: true,
    });
  });
});
