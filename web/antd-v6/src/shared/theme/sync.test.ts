import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ThemeScopeResource } from './contract';
import {
  closeThemeSyncTransport,
  decideThemeScopeEvent,
  parseThemeSyncEvent,
  publishThemeScopeResource,
  subscribeThemeSync,
  THEME_SYNC_EVENT_TTL_MS,
  THEME_SYNC_STORAGE_KEY,
  type ThemeScopeUpdatedEvent,
} from './sync';

class FakeBroadcastChannel {
  static instances: FakeBroadcastChannel[] = [];
  listeners: Array<(event: MessageEvent) => void> = [];
  posted: unknown[] = [];
  failPost = false;

  constructor(public readonly name: string) {
    FakeBroadcastChannel.instances.push(this);
  }

  addEventListener(_type: string, listener: (event: MessageEvent) => void) {
    this.listeners.push(listener);
  }

  postMessage(value: unknown) {
    if (this.failPost) throw new Error('channel unavailable');
    this.posted.push(value);
  }

  close() {}
}

function userEvent(overrides = { fixedHeader: true }): ThemeScopeUpdatedEvent {
  return {
    v: 1,
    id: 'other-tab:1',
    origin: 'other-tab',
    issuedAt: Date.now(),
    type: 'scope-updated',
    scope: 'user',
    revision: '9',
    overrides,
    authSessionId: 'session-a',
  };
}

describe('v6 theme cross-tab synchronization', () => {
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
    vi.restoreAllMocks();
    Object.defineProperty(window, 'BroadcastChannel', {
      configurable: true,
      value: originalBroadcastChannel,
    });
  });

  it('validates schema, TTL, revision, and personal session binding', () => {
    const now = Date.now();
    expect(parseThemeSyncEvent(userEvent(), now)).toMatchObject({ revision: '9' });
    expect(
      parseThemeSyncEvent({ ...userEvent(), issuedAt: now - THEME_SYNC_EVENT_TTL_MS - 1 }, now),
    ).toBeUndefined();
    expect(parseThemeSyncEvent({ ...userEvent(), revision: '1e3' }, now)).toBeUndefined();
    expect(parseThemeSyncEvent({ ...userEvent(), authSessionId: undefined }, now)).toBeUndefined();

    const current: ThemeScopeResource = {
      scope: 'user',
      revision: '8',
      overrides: {},
      versioned: true,
    };
    expect(decideThemeScopeEvent(userEvent(), current, 'session-a')).toBe('apply');
    expect(decideThemeScopeEvent(userEvent(), current, 'session-b')).toBe('wrong-session');
    expect(
      decideThemeScopeEvent({ ...userEvent(), revision: '8', overrides: {} }, current, 'session-a'),
    ).toBe('duplicate');
    expect(
      decideThemeScopeEvent(
        { ...userEvent(), revision: '8', overrides: { colorWeak: true } },
        current,
        'session-a',
      ),
    ).toBe('conflict');
  });

  it('dispatches locally once and publishes through BroadcastChannel', () => {
    const listener = vi.fn();
    const storageWrite = vi.spyOn(Storage.prototype, 'setItem');
    const unsubscribe = subscribeThemeSync(listener);

    publishThemeScopeResource({
      scope: 'application',
      revision: '12',
      overrides: { navTheme: 'light' },
      versioned: true,
    });

    expect(listener).toHaveBeenCalledTimes(1);
    expect(FakeBroadcastChannel.instances[0]?.posted).toHaveLength(1);
    expect(storageWrite).not.toHaveBeenCalled();
    unsubscribe();
  });

  it('uses an ephemeral storage event only when channel sending fails', () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    const removeItem = vi.spyOn(Storage.prototype, 'removeItem');
    const unsubscribe = subscribeThemeSync(vi.fn());
    const channel = FakeBroadcastChannel.instances[0];
    if (!channel) throw new Error('Fake channel did not start');
    channel.failPost = true;

    publishThemeScopeResource({
      scope: 'application',
      revision: '13',
      overrides: { colorWeak: true },
      versioned: true,
    });

    expect(setItem).toHaveBeenCalledWith(THEME_SYNC_STORAGE_KEY, expect.any(String));
    expect(removeItem).toHaveBeenCalledWith(THEME_SYNC_STORAGE_KEY);
    unsubscribe();
  });

  it('deduplicates repeated storage delivery', () => {
    const listener = vi.fn();
    const unsubscribe = subscribeThemeSync(listener);
    const payload = JSON.stringify(userEvent());
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
