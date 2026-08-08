import {
  closeThemeSyncTransport,
  parseThemeSyncEvent,
  publishThemeScopeResource,
  shouldReconcileThemeScopeEvent,
  subscribeThemeSync,
  THEME_SYNC_EVENT_TTL_MS,
  THEME_SYNC_STORAGE_KEY,
  type ThemeScopeUpdatedEvent,
} from './themeSync';

class FakeBroadcastChannel {
  static instances: FakeBroadcastChannel[] = [];

  listeners: Array<(event: MessageEvent) => void> = [];
  posted: unknown[] = [];
  closed = false;
  failPost = false;

  constructor(public name: string) {
    FakeBroadcastChannel.instances.push(this);
  }

  addEventListener(_type: string, listener: (event: MessageEvent) => void) {
    this.listeners.push(listener);
  }

  postMessage(value: unknown) {
    if (this.failPost) throw new Error('channel unavailable');
    this.posted.push(value);
  }

  close() {
    this.closed = true;
  }
}

const event = (overrides = { fixedHeader: true }): ThemeScopeUpdatedEvent => ({
  v: 1,
  id: 'other-tab:1',
  origin: 'other-tab',
  issuedAt: Date.now(),
  type: 'scope-updated',
  scope: 'user',
  revision: '9',
  overrides,
  authSessionId: 'session-a',
});

describe('theme cross-tab synchronization', () => {
  const originalBroadcastChannel = window.BroadcastChannel;

  beforeEach(() => {
    FakeBroadcastChannel.instances = [];
    Object.defineProperty(window, 'BroadcastChannel', {
      configurable: true,
      value: FakeBroadcastChannel,
    });
  });

  afterEach(() => {
    closeThemeSyncTransport();
    jest.restoreAllMocks();
    Object.defineProperty(window, 'BroadcastChannel', {
      configurable: true,
      value: originalBroadcastChannel,
    });
  });

  it('validates schema, TTL, revisions, and user session binding', () => {
    const now = Date.now();
    expect(parseThemeSyncEvent(event(), now)).toMatchObject({ revision: '9' });
    expect(
      parseThemeSyncEvent({ ...event(), issuedAt: now - THEME_SYNC_EVENT_TTL_MS - 1 }, now),
    ).toBeUndefined();
    expect(parseThemeSyncEvent({ ...event(), authSessionId: undefined }, now)).toBeUndefined();
    expect(parseThemeSyncEvent({ ...event(), revision: '1e3' }, now)).toBeUndefined();

    const current = {
      scope: 'user' as const,
      revision: '8',
      overrides: {},
      versioned: true,
    };
    expect(shouldReconcileThemeScopeEvent(event(), current, 'session-a')).toBe(true);
    expect(shouldReconcileThemeScopeEvent(event(), current, 'session-b')).toBe(false);
    expect(
      shouldReconcileThemeScopeEvent(
        { ...event(), revision: '8', overrides: {} },
        current,
        'session-a',
      ),
    ).toBe(false);
    expect(
      shouldReconcileThemeScopeEvent(
        { ...event(), revision: '8', overrides: { fixedHeader: true } },
        current,
        'session-a',
      ),
    ).toBe(true);
  });

  it('publishes through BroadcastChannel and dispatches locally once', () => {
    const listener = jest.fn();
    const storage = jest.spyOn(Storage.prototype, 'setItem');
    const unsubscribe = subscribeThemeSync(listener);

    publishThemeScopeResource(
      {
        scope: 'application',
        revision: '12',
        overrides: { navTheme: 'light' },
        versioned: true,
      },
      undefined,
    );

    expect(listener).toHaveBeenCalledTimes(1);
    expect(FakeBroadcastChannel.instances[0].posted).toHaveLength(1);
    expect(storage).not.toHaveBeenCalled();
    unsubscribe();
  });

  it('falls back to an ephemeral storage event when channel sending fails', () => {
    const setItem = window.localStorage.setItem as jest.Mock;
    const removeItem = window.localStorage.removeItem as jest.Mock;
    setItem.mockClear();
    removeItem.mockClear();
    const unsubscribe = subscribeThemeSync(jest.fn());
    FakeBroadcastChannel.instances[0].failPost = true;

    publishThemeScopeResource(
      {
        scope: 'application',
        revision: '13',
        overrides: { colorWeak: true },
        versioned: true,
      },
      undefined,
    );

    expect(setItem).toHaveBeenCalledWith(THEME_SYNC_STORAGE_KEY, expect.any(String));
    expect(removeItem).toHaveBeenCalledWith(THEME_SYNC_STORAGE_KEY);
    unsubscribe();
  });

  it('deduplicates a storage event delivered more than once', () => {
    const listener = jest.fn();
    const unsubscribe = subscribeThemeSync(listener);
    const payload = JSON.stringify(event());

    window.dispatchEvent(
      new StorageEvent('storage', { key: THEME_SYNC_STORAGE_KEY, newValue: payload }),
    );
    window.dispatchEvent(
      new StorageEvent('storage', { key: THEME_SYNC_STORAGE_KEY, newValue: payload }),
    );

    expect(listener).toHaveBeenCalledTimes(1);
    unsubscribe();
  });
});
