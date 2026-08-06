import { prepareAPIRequest } from './apiRequest';

describe('API request credentials contract', () => {
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
});
