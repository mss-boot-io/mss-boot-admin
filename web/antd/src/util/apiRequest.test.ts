import { prepareAPIRequest } from './apiRequest';
import { clearTransientAuthToken, setTransientAuthToken } from '@/utils/authStorage';

describe('API request credentials contract', () => {
  afterEach(() => {
    clearTransientAuthToken();
  });

  it('includes cross-origin credentials and preserves caller headers', () => {
    expect(
      prepareAPIRequest(
        '/admin/api/user/oauth2/authorize',
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'X-Request-ID': 'request-1' },
        },
        'https://admin-api.mss-boot-io.top',
        'session-token',
      ),
    ).toEqual({
      url: 'https://admin-api.mss-boot-io.top/admin/api/user/oauth2/authorize',
      options: {
        method: 'POST',
        withCredentials: true,
        headers: {
          'Content-Type': 'application/json',
          'X-Request-ID': 'request-1',
          Authorization: 'Bearer session-token',
        },
      },
    });
  });

  it('includes credentials for anonymous OAuth callbacks', () => {
    expect(
      prepareAPIRequest(
        '/admin/api/user/github/callback?code=code&state=state',
        { method: 'GET' },
        'https://admin-api.mss-boot-io.top',
        null,
      ),
    ).toEqual({
      url: 'https://admin-api.mss-boot-io.top/admin/api/user/github/callback?code=code&state=state',
      options: {
        method: 'GET',
        withCredentials: true,
      },
    });
  });

  it('uses the document-scoped OAuth bearer for authenticated API requests', () => {
    setTransientAuthToken('oauth-admin-token');

    expect(
      prepareAPIRequest(
        '/admin/api/user/profile',
        { method: 'GET' },
        'https://admin-api.mss-boot-io.top',
      ),
    ).toEqual({
      url: 'https://admin-api.mss-boot-io.top/admin/api/user/profile',
      options: {
        method: 'GET',
        withCredentials: true,
        headers: { Authorization: 'Bearer oauth-admin-token' },
      },
    });
  });

  it('omits a stale Admin bearer token from login-intent OAuth authorization', () => {
    expect(
      prepareAPIRequest(
        '/admin/api/user/oauth2/authorize',
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          skipAuthToken: true,
        } as any,
        'https://admin-api.mss-boot-io.top',
        'stale-admin-token',
      ),
    ).toEqual({
      url: 'https://admin-api.mss-boot-io.top/admin/api/user/oauth2/authorize',
      options: {
        method: 'POST',
        withCredentials: true,
        headers: { 'Content-Type': 'application/json' },
      },
    });
  });

  it('omits the document-scoped bearer when a request skips authentication', () => {
    setTransientAuthToken('oauth-admin-token');

    expect(
      prepareAPIRequest(
        '/admin/api/user/oauth2/authorize',
        { method: 'POST', skipAuthToken: true } as any,
        'https://admin-api.mss-boot-io.top',
      ),
    ).toEqual({
      url: 'https://admin-api.mss-boot-io.top/admin/api/user/oauth2/authorize',
      options: { method: 'POST', withCredentials: true },
    });
  });
});
