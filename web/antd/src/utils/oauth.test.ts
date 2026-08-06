import { postUserOauth2Authorize } from '@/services/admin/user';
import { openOAuthAuthorization, requireServerOAuthIntent } from './oauth';

jest.mock('@/services/admin/user', () => ({
  postUserOauth2Authorize: jest.fn(),
}));

const mockedAuthorize = postUserOauth2Authorize as jest.Mock;

describe('openOAuthAuthorization', () => {
  const originalOpen = window.open;

  afterEach(() => {
    window.open = originalOpen;
    jest.clearAllMocks();
  });

  it('uses the server-issued URL and does not accept a client state', async () => {
    const popup = {
      close: jest.fn(),
      location: { href: 'about:blank' },
      opener: window,
    } as unknown as Window;
    window.open = jest.fn(() => popup);
    mockedAuthorize.mockResolvedValue({
      authorizeURL: 'https://github.example/authorize?state=server-state',
      expiresAt: '2026-08-06T12:05:00Z',
    });

    await openOAuthAuthorization('github', 'binding');

    expect(mockedAuthorize).toHaveBeenCalledWith({ provider: 'github', intent: 'binding' });
    expect(popup.opener).toBeNull();
    expect(popup.location.href).toBe('https://github.example/authorize?state=server-state');
  });

  it('closes the placeholder popup when issuance fails', async () => {
    const popup = {
      close: jest.fn(),
      location: { href: 'about:blank' },
      opener: window,
    } as unknown as Window;
    window.open = jest.fn(() => popup);
    mockedAuthorize.mockRejectedValue(new Error('unavailable'));

    await expect(openOAuthAuthorization('lark', 'login')).rejects.toThrow('unavailable');
    expect(popup.close).toHaveBeenCalled();
  });
});

describe('requireServerOAuthIntent', () => {
  it('uses only the server-returned callback intent', () => {
    expect(requireServerOAuthIntent({ intent: 'login' })).toBe('login');
    expect(requireServerOAuthIntent({ intent: 'binding' })).toBe('binding');
    expect(() => requireServerOAuthIntent({ intent: 'integration' as API.OAuthIntent })).toThrow();
    expect(() => requireServerOAuthIntent({ intent: 'invalid' as API.OAuthIntent })).toThrow();
  });
});
