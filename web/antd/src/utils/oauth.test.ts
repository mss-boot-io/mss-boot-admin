import { postUserClearAuthCookie, postUserOauth2Authorize } from '@/services/admin/user';
import {
  LEGACY_OAUTH_STORAGE_KEYS,
  OAuthAuthorizationError,
  getOAuthChannelName,
  openOAuthAuthorization,
  publishOAuthCallbackResult,
  purgeLegacyOAuthStorage,
  requireServerOAuthIntent,
} from './oauth';

jest.mock('@/services/admin/user', () => ({
  postUserClearAuthCookie: jest.fn(),
  postUserOauth2Authorize: jest.fn(),
}));

const mockedClearAuthCookie = postUserClearAuthCookie as jest.Mock;
const mockedAuthorize = postUserOauth2Authorize as jest.Mock;

class FakeBroadcastChannel {
  static instances: FakeBroadcastChannel[] = [];

  name: string;
  closed = false;
  listeners = new Set<(event: MessageEvent<unknown>) => void>();
  postMessage = jest.fn((data: unknown) => {
    FakeBroadcastChannel.instances
      .filter((channel) => channel !== this && channel.name === this.name && !channel.closed)
      .forEach((channel) => channel.emit(data));
  });

  constructor(name: string) {
    this.name = name;
    FakeBroadcastChannel.instances.push(this);
  }

  addEventListener(_type: 'message', listener: (event: MessageEvent<unknown>) => void) {
    this.listeners.add(listener);
  }

  removeEventListener(_type: 'message', listener: (event: MessageEvent<unknown>) => void) {
    this.listeners.delete(listener);
  }

  emit(data: unknown) {
    [...this.listeners].forEach((listener) => listener({ data } as MessageEvent<unknown>));
  }

  close() {
    this.closed = true;
  }
}

function createPopup(onNavigate?: (href: string) => void) {
  let closed = false;
  let href = 'about:blank';
  const location = {} as Location;
  Object.defineProperty(location, 'href', {
    configurable: true,
    get: () => href,
    set: (value: string) => {
      href = value;
      onNavigate?.(value);
    },
  });
  return {
    close: jest.fn(() => {
      closed = true;
    }),
    get closed() {
      return closed;
    },
    location,
    opener: window,
  } as unknown as Window;
}

async function flushAuthorization() {
  await Promise.resolve();
  await Promise.resolve();
}

describe('openOAuthAuthorization', () => {
  const originalOpen = window.open;
  const originalBroadcastChannel = global.BroadcastChannel;

  beforeEach(() => {
    FakeBroadcastChannel.instances = [];
    Object.defineProperty(global, 'BroadcastChannel', {
      configurable: true,
      value: FakeBroadcastChannel,
    });
    localStorage.clear();
    jest.clearAllMocks();
  });

  afterEach(() => {
    window.open = originalOpen;
    Object.defineProperty(global, 'BroadcastChannel', {
      configurable: true,
      value: originalBroadcastChannel,
    });
    jest.useRealTimers();
  });

  it('opens synchronously, then listens before provider navigation and returns only the Admin login result', async () => {
    const events: string[] = [];
    const popup = createPopup(() => events.push('navigate'));
    const originalAddEventListener = FakeBroadcastChannel.prototype.addEventListener;
    jest
      .spyOn(FakeBroadcastChannel.prototype, 'addEventListener')
      .mockImplementation(function (this: FakeBroadcastChannel, type, listener) {
        events.push('listen');
        originalAddEventListener.call(this, type, listener);
      });
    window.open = jest.fn(() => {
      events.push('open');
      return popup;
    });
    let resolveAuthorize!: (value: API.OAuthAuthorizeResponse) => void;
    mockedAuthorize.mockImplementation(
      () =>
        new Promise<API.OAuthAuthorizeResponse>((resolve) => {
          resolveAuthorize = resolve;
        }),
    );

    localStorage.setItem('token', 'stale-admin-token');
    localStorage.setItem('token.expire', 'expired');
    localStorage.setItem('autoLogin', 'false');
    jest.clearAllMocks();

    const pending = openOAuthAuthorization('github', 'login');
    expect(window.open).toHaveBeenCalledWith('about:blank', '_blank');
    expect(events).toEqual(['open']);
    await flushAuthorization();
    resolveAuthorize({
      attemptID: 'attempt-login',
      authorizeURL: 'https://github.example/authorize?state=server-state',
      expiresAt: new Date(Date.now() + 60_000).toISOString(),
    });
    await flushAuthorization();
    expect(events).toEqual(['open', 'listen', 'navigate']);
    publishOAuthCallbackResult({
      attemptID: 'attempt-login',
      code: 200,
      provider: 'github',
      intent: 'login',
      token: 'admin-session-token',
      expire: '2026-08-06T12:05:00Z',
    });

    await expect(pending).resolves.toEqual({
      attemptID: 'attempt-login',
      code: 200,
      provider: 'github',
      intent: 'login',
      token: 'admin-session-token',
      expire: '2026-08-06T12:05:00Z',
    });
    expect(mockedAuthorize).toHaveBeenCalledWith(
      { provider: 'github', intent: 'login' },
      { skipAuthToken: true },
    );
    expect(mockedClearAuthCookie).toHaveBeenCalledWith({
      skipAuthToken: true,
      skipErrorHandler: true,
    });
    expect(mockedClearAuthCookie.mock.invocationCallOrder[0]).toBeLessThan(
      mockedAuthorize.mock.invocationCallOrder[0],
    );
    expect(popup.opener).toBeNull();
    expect(popup.location.href).toBe(
      'https://github.example/authorize?state=server-state',
    );
    expect(localStorage.setItem).not.toHaveBeenCalled();
    expect(localStorage.removeItem).toHaveBeenCalledWith('token');
    expect(localStorage.removeItem).toHaveBeenCalledWith('token.expire');
    expect(localStorage.removeItem).toHaveBeenCalledWith('autoLogin');
    expect(FakeBroadcastChannel.instances.every((channel) => channel.closed)).toBe(true);
  });

  it('returns only the opaque integration credential handle', async () => {
    const popup = createPopup();
    window.open = jest.fn(() => popup);
    mockedAuthorize.mockResolvedValue({
      attemptID: 'attempt-integration',
      authorizeURL: 'https://github.example/authorize',
      expiresAt: new Date(Date.now() + 60_000).toISOString(),
    });

    const pending = openOAuthAuthorization('github', 'integration');
    await flushAuthorization();
    publishOAuthCallbackResult({
      attemptID: 'attempt-integration',
      code: 200,
      provider: 'github',
      intent: 'integration',
      credential: 'opaque-handle',
      credentialExpiresAt: '2026-08-06T12:05:00Z',
    });

    await expect(pending).resolves.toEqual({
      attemptID: 'attempt-integration',
      code: 200,
      provider: 'github',
      intent: 'integration',
      credential: 'opaque-handle',
      credentialExpiresAt: '2026-08-06T12:05:00Z',
    });
    expect(JSON.stringify(FakeBroadcastChannel.instances[1].postMessage.mock.calls)).not.toContain(
      'accessToken',
    );
  });

  it('rejects a callback that contains provider credential fields before broadcasting', () => {
    expect(() =>
      publishOAuthCallbackResult({
        attemptID: 'attempt-leak',
        code: 200,
        provider: 'github',
        intent: 'integration',
        credential: 'opaque-handle',
        accessToken: 'provider-secret',
      } as API.OAuthCallbackResponse),
    ).toThrow(OAuthAuthorizationError);
    expect(FakeBroadcastChannel.instances).toHaveLength(0);
  });

  it('cleans up when the popup is blocked', async () => {
    window.open = jest.fn(() => null);
    mockedAuthorize.mockResolvedValue({
      attemptID: 'attempt-blocked',
      authorizeURL: 'https://github.example/authorize',
      expiresAt: new Date(Date.now() + 60_000).toISOString(),
    });

    await expect(openOAuthAuthorization('github', 'binding')).rejects.toMatchObject({
      code: 'popup-blocked',
    });
    expect(mockedAuthorize).not.toHaveBeenCalled();
    expect(FakeBroadcastChannel.instances).toHaveLength(0);
  });

  it('closes the placeholder popup when attempt issuance fails', async () => {
    const popup = createPopup();
    window.open = jest.fn(() => popup);
    mockedAuthorize.mockRejectedValue(new Error('unavailable'));

    await expect(openOAuthAuthorization('lark', 'login')).rejects.toThrow('unavailable');
    expect(popup.close).toHaveBeenCalled();
    expect(FakeBroadcastChannel.instances).toHaveLength(0);
  });

  it('cleans up when authorization times out', async () => {
    jest.useFakeTimers();
    const popup = createPopup();
    window.open = jest.fn(() => popup);
    mockedAuthorize.mockResolvedValue({
      attemptID: 'attempt-timeout',
      authorizeURL: 'https://github.example/authorize',
      expiresAt: new Date(Date.now() + 1000).toISOString(),
    });

    const pending = openOAuthAuthorization('github', 'binding');
    await flushAuthorization();
    jest.advanceTimersByTime(1000);

    await expect(pending).rejects.toMatchObject({ code: 'timeout' });
    expect(popup.close).toHaveBeenCalled();
    expect(FakeBroadcastChannel.instances[0].closed).toBe(true);
  });

  it('cleans up when the user closes the popup', async () => {
    jest.useFakeTimers();
    const popup = createPopup();
    window.open = jest.fn(() => popup);
    mockedAuthorize.mockResolvedValue({
      attemptID: 'attempt-closed',
      authorizeURL: 'https://github.example/authorize',
      expiresAt: new Date(Date.now() + 60_000).toISOString(),
    });

    const pending = openOAuthAuthorization('github', 'binding');
    await flushAuthorization();
    popup.close();
    jest.advanceTimersByTime(250);

    await expect(pending).rejects.toMatchObject({ code: 'closed' });
    expect(FakeBroadcastChannel.instances[0].closed).toBe(true);
  });
});

describe('OAuth guards', () => {
  it('uses only the server-returned callback intent', () => {
    expect(requireServerOAuthIntent({ intent: 'login' })).toBe('login');
    expect(requireServerOAuthIntent({ intent: 'binding' })).toBe('binding');
    expect(requireServerOAuthIntent({ intent: 'integration' })).toBe('integration');
    expect(() =>
      requireServerOAuthIntent({ intent: 'invalid' as API.OAuthIntent }),
    ).toThrow();
  });

  it('binds the channel name to the server attempt ID', () => {
    expect(getOAuthChannelName('attempt-123')).toBe('mss-admin-oauth:attempt-123');
  });

  it('purges only legacy provider-flow keys and keeps the Admin session', () => {
    purgeLegacyOAuthStorage();

    LEGACY_OAUTH_STORAGE_KEYS.forEach((key) => {
      expect(localStorage.removeItem).toHaveBeenCalledWith(key);
    });
    expect(localStorage.removeItem).not.toHaveBeenCalledWith('token');
    expect(localStorage.removeItem).not.toHaveBeenCalledWith('token.expire');
  });
});
