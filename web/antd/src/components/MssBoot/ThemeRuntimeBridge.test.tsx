import { act, render, waitFor } from '@testing-library/react';
import * as React from 'react';
import { getAppConfigsProfile } from '@/services/admin/appConfig';
import { getUserConfigsProfile } from '@/services/admin/userConfig';
import {
  readThemeSnapshot,
  THEME_AUTH_SESSION_KEY,
  writeThemeSnapshot,
} from '@/utils/themeSession';
import ThemeRuntimeBridge, { THEME_RECONCILE_TIMEOUT_MS } from './ThemeRuntimeBridge';

let mockRuntimeState: any;
let mockSyncListener: ((event: any) => void) | undefined;
const mockHistoryGo = jest.fn();
const mockSetInitialState = jest.fn((updater) => {
  mockRuntimeState = typeof updater === 'function' ? updater(mockRuntimeState) : updater;
});
const originalLocksDescriptor = Object.getOwnPropertyDescriptor(navigator, 'locks');

jest.mock('@umijs/max', () => ({
  history: { go: (delta: number) => mockHistoryGo(delta) },
  useModel: () => ({ initialState: mockRuntimeState, setInitialState: mockSetInitialState }),
}));

jest.mock('@/services/admin/appConfig', () => ({
  getAppConfigsProfile: jest.fn(),
}));

jest.mock('@/services/admin/userConfig', () => ({
  getUserConfigsProfile: jest.fn(),
}));

jest.mock('@/utils/themeSync', () => {
  const actual = jest.requireActual('@/utils/themeSync');
  return {
    ...actual,
    subscribeThemeSync: jest.fn((listener) => {
      mockSyncListener = listener;
      return jest.fn();
    }),
  };
});

describe('ThemeRuntimeBridge', () => {
  const storageValues = new Map<string, string>();

  beforeEach(() => {
    jest.clearAllMocks();
    Object.defineProperty(navigator, 'locks', {
      configurable: true,
      value: {
        request: jest.fn((_name: string, callback: () => unknown) =>
          Promise.resolve().then(callback),
        ),
      },
    });
    storageValues.clear();
    (localStorage.getItem as jest.Mock).mockImplementation(
      (key: string) => storageValues.get(key) ?? null,
    );
    (localStorage.setItem as jest.Mock).mockImplementation((key: string, value: string) => {
      storageValues.set(key, String(value));
    });
    (localStorage.removeItem as jest.Mock).mockImplementation((key: string) => {
      storageValues.delete(key);
    });
    mockSyncListener = undefined;
    document.documentElement.removeAttribute('data-mss-theme');
    document.documentElement.style.removeProperty('color-scheme');
    document.documentElement.style.removeProperty('--mss-theme-color-primary');
    mockRuntimeState = {
      appConfig: {
        theme: {
          fixedHeader: false,
          _meta: { v: 1, scope: 'application', revision: '1' },
        },
      },
      settings: { fixedHeader: false },
      themeRuntime: {
        schemaVersion: 1,
        layers: {
          application: {
            scope: 'application',
            revision: '1',
            overrides: { fixedHeader: false },
            versioned: true,
          },
        },
      },
    };
  });

  afterEach(() => {
    jest.useRealTimers();
    if (originalLocksDescriptor) {
      Object.defineProperty(navigator, 'locks', originalLocksDescriptor);
    } else {
      Reflect.deleteProperty(navigator, 'locks');
    }
  });

  it('keeps document color hints aligned with the final effective runtime theme', async () => {
    mockRuntimeState.settings = {
      ...mockRuntimeState.settings,
      navTheme: 'realDark',
      colorPrimary: '#112233',
    };
    const view = render(<ThemeRuntimeBridge />);

    await waitFor(() => expect(document.documentElement.style.colorScheme).toBe('dark'));
    expect(document.documentElement.dataset.mssTheme).toBe('realDark');
    expect(document.documentElement.style.getPropertyValue('--mss-theme-color-primary')).toBe(
      '#112233',
    );

    mockRuntimeState = {
      ...mockRuntimeState,
      settings: {
        ...mockRuntimeState.settings,
        navTheme: 'light',
        colorPrimary: '#aabbcc',
      },
    };
    view.rerender(<ThemeRuntimeBridge />);

    await waitFor(() => expect(document.documentElement.style.colorScheme).toBe('light'));
    expect(document.documentElement.dataset.mssTheme).toBe('light');
    expect(document.documentElement.style.getPropertyValue('--mss-theme-color-primary')).toBe(
      '#aabbcc',
    );
  });

  it('persists snapshots from the final rendered runtime layer', async () => {
    const view = render(<ThemeRuntimeBridge />);
    await waitFor(() => expect(readThemeSnapshot('application')?.revision).toBe('1'));

    const application = {
      scope: 'application' as const,
      revision: '3',
      overrides: { fixedHeader: true },
      versioned: true,
    };
    mockRuntimeState = {
      ...mockRuntimeState,
      appConfig: {
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '3' },
        },
      },
      themeRuntime: {
        ...mockRuntimeState.themeRuntime,
        layers: { application },
      },
    };
    view.rerender(<ThemeRuntimeBridge />);

    await waitFor(() => expect(readThemeSnapshot('application')?.revision).toBe('3'));
    expect(readThemeSnapshot('application')?.overrides).toEqual({ fixedHeader: true });
  });

  it('replaces an exact degraded warm hint with the lower authoritative revision', async () => {
    const warmHint = {
      scope: 'application' as const,
      revision: '12',
      overrides: { fixedHeader: true },
      versioned: true,
    };
    await writeThemeSnapshot(warmHint);
    mockRuntimeState = {
      appConfig: {
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '12' },
        },
      },
      settings: { fixedHeader: true },
      themeRuntime: {
        schemaVersion: 1,
        layers: { application: warmHint },
        degradedScopes: ['application'],
      },
    };
    (getAppConfigsProfile as jest.Mock).mockResolvedValue({
      theme: {
        fixedHeader: false,
        _meta: { v: 1, scope: 'application', revision: '10' },
      },
    });
    const view = render(<ThemeRuntimeBridge />);

    act(() => {
      mockSyncListener?.({
        v: 1,
        id: 'tab-a:13',
        origin: 'tab-a',
        issuedAt: Date.now(),
        type: 'scope-updated',
        scope: 'application',
        revision: '13',
        overrides: {},
      });
    });

    await waitFor(() =>
      expect(mockRuntimeState.themeRuntime.layers.application.revision).toBe('10'),
    );
    view.rerender(<ThemeRuntimeBridge />);
    await waitFor(() => expect(readThemeSnapshot('application')?.revision).toBe('10'));
  });

  it('carries the original warm hint across consecutive authoritative updates before commit', async () => {
    const warmHint = {
      scope: 'application' as const,
      revision: '12',
      overrides: { fixedHeader: true },
      versioned: true,
    };
    await writeThemeSnapshot(warmHint);
    mockRuntimeState = {
      appConfig: {
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '12' },
        },
      },
      settings: { fixedHeader: true },
      themeRuntime: {
        schemaVersion: 1,
        layers: { application: warmHint },
        degradedScopes: ['application'],
      },
    };
    let resolveFirst!: (profile: Record<string, unknown>) => void;
    let resolveSecond!: (profile: Record<string, unknown>) => void;
    (getAppConfigsProfile as jest.Mock)
      .mockReturnValueOnce(
        new Promise<Record<string, unknown>>((resolve) => {
          resolveFirst = resolve;
        }),
      )
      .mockReturnValueOnce(
        new Promise<Record<string, unknown>>((resolve) => {
          resolveSecond = resolve;
        }),
      );
    const view = render(<ThemeRuntimeBridge />);
    await waitFor(() => expect(getAppConfigsProfile).toHaveBeenCalledTimes(1));

    act(() => {
      mockSyncListener?.({
        v: 1,
        id: 'tab-a:13',
        origin: 'tab-a',
        issuedAt: Date.now(),
        type: 'scope-updated',
        scope: 'application',
        revision: '13',
        overrides: {},
      });
      resolveFirst({
        theme: {
          fixedHeader: false,
          _meta: { v: 1, scope: 'application', revision: '10' },
        },
      });
    });
    await waitFor(() => expect(getAppConfigsProfile).toHaveBeenCalledTimes(2));
    act(() => {
      resolveSecond({
        theme: {
          fixedHeader: false,
          colorWeak: true,
          _meta: { v: 1, scope: 'application', revision: '11' },
        },
      });
    });

    await waitFor(() =>
      expect(mockRuntimeState.themeRuntime.layers.application.revision).toBe('11'),
    );
    view.rerender(<ThemeRuntimeBridge />);
    await waitFor(() => expect(readThemeSnapshot('application')?.revision).toBe('11'));
    expect(readThemeSnapshot('application')?.overrides).toEqual({
      fixedHeader: false,
      colorWeak: true,
    });
  });

  it('does not let a degraded warm layer overwrite another tab authoritative snapshot', async () => {
    const authoritative = {
      scope: 'application' as const,
      revision: '10',
      overrides: { fixedHeader: false },
      versioned: true,
    };
    await writeThemeSnapshot(authoritative);
    const warmHint = {
      scope: 'application' as const,
      revision: '12',
      overrides: { fixedHeader: true },
      versioned: true,
    };
    mockRuntimeState = {
      appConfig: {
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '12' },
        },
      },
      settings: { fixedHeader: true },
      themeRuntime: {
        schemaVersion: 1,
        layers: { application: warmHint },
        degradedScopes: ['application'],
      },
    };
    let resolveProfile!: (profile: Record<string, unknown>) => void;
    (getAppConfigsProfile as jest.Mock).mockReturnValue(
      new Promise<Record<string, unknown>>((resolve) => {
        resolveProfile = resolve;
      }),
    );
    const view = render(<ThemeRuntimeBridge />);
    await waitFor(() => expect(getAppConfigsProfile).toHaveBeenCalledTimes(1));
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(readThemeSnapshot('application')).toEqual(authoritative);

    act(() => {
      resolveProfile({
        theme: {
          fixedHeader: false,
          _meta: { v: 1, scope: 'application', revision: '10' },
        },
      });
    });
    await waitFor(() =>
      expect(mockRuntimeState.themeRuntime.layers.application.revision).toBe('10'),
    );
    view.rerender(<ThemeRuntimeBridge />);
    await waitFor(() => expect(readThemeSnapshot('application')).toEqual(authoritative));
  });

  it('does not let an old authoritative response overwrite a rev11 snapshot', async () => {
    const warmHint = {
      scope: 'application' as const,
      revision: '12',
      overrides: { fixedHeader: true },
      versioned: true,
    };
    await writeThemeSnapshot(warmHint);
    mockRuntimeState = {
      appConfig: {
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '12' },
        },
      },
      settings: { fixedHeader: true },
      themeRuntime: {
        schemaVersion: 1,
        layers: { application: warmHint },
        degradedScopes: ['application'],
      },
    };
    let resolveProfile!: (profile: Record<string, unknown>) => void;
    (getAppConfigsProfile as jest.Mock).mockReturnValue(
      new Promise<Record<string, unknown>>((resolve) => {
        resolveProfile = resolve;
      }),
    );
    const view = render(<ThemeRuntimeBridge />);

    act(() => {
      mockSyncListener?.({
        v: 1,
        id: 'tab-a:13',
        origin: 'tab-a',
        issuedAt: Date.now(),
        type: 'scope-updated',
        scope: 'application',
        revision: '13',
        overrides: {},
      });
    });
    await waitFor(() => expect(getAppConfigsProfile).toHaveBeenCalledTimes(1));

    const rev11 = {
      scope: 'application' as const,
      revision: '11',
      overrides: { fixedHeader: false, colorWeak: true },
      versioned: true,
    };
    expect(
      await writeThemeSnapshot(rev11, undefined, Date.now(), {
        authoritativePrevious: warmHint,
      }),
    ).toBe(true);
    resolveProfile({
      theme: {
        fixedHeader: false,
        _meta: { v: 1, scope: 'application', revision: '10' },
      },
    });

    await waitFor(() =>
      expect(mockRuntimeState.themeRuntime.layers.application.revision).toBe('10'),
    );
    view.rerender(<ThemeRuntimeBridge />);
    await waitFor(() => expect(readThemeSnapshot('application')?.revision).toBe('11'));
  });

  it('treats a cross-tab payload as invalidation and applies only the authoritative response', async () => {
    let resolveProfile!: (profile: Record<string, unknown>) => void;
    (getAppConfigsProfile as jest.Mock).mockReturnValue(
      new Promise<Record<string, unknown>>((resolve) => {
        resolveProfile = resolve;
      }),
    );
    render(<ThemeRuntimeBridge />);

    act(() => {
      mockSyncListener?.({
        v: 1,
        id: 'untrusted-tab:1',
        origin: 'untrusted-tab',
        issuedAt: Date.now(),
        type: 'scope-updated',
        scope: 'application',
        revision: '999999999999999999999',
        overrides: { fixedHeader: true },
      });
    });

    expect(mockRuntimeState.themeRuntime.layers.application).toMatchObject({
      revision: '1',
      overrides: { fixedHeader: false },
    });
    resolveProfile({
      theme: {
        fixedHeader: false,
        colorWeak: true,
        _meta: { v: 1, scope: 'application', revision: '1' },
      },
    });

    await waitFor(() =>
      expect(mockRuntimeState.themeRuntime.layers.application).toMatchObject({
        revision: '1',
        overrides: { fixedHeader: false, colorWeak: true },
      }),
    );
    expect(mockRuntimeState.settings).toMatchObject({ fixedHeader: false, colorWeak: true });
  });

  it('queues another authoritative read when a newer event arrives during reconciliation', async () => {
    let resolveFirst!: (profile: Record<string, unknown>) => void;
    let resolveSecond!: (profile: Record<string, unknown>) => void;
    (getAppConfigsProfile as jest.Mock)
      .mockReturnValueOnce(
        new Promise<Record<string, unknown>>((resolve) => {
          resolveFirst = resolve;
        }),
      )
      .mockReturnValueOnce(
        new Promise<Record<string, unknown>>((resolve) => {
          resolveSecond = resolve;
        }),
      );
    render(<ThemeRuntimeBridge />);

    act(() => {
      mockSyncListener?.({
        v: 1,
        id: 'tab-a:2',
        origin: 'tab-a',
        issuedAt: Date.now(),
        type: 'scope-updated',
        scope: 'application',
        revision: '2',
        overrides: {},
      });
    });
    await waitFor(() => expect(getAppConfigsProfile).toHaveBeenCalledTimes(1));

    act(() => {
      mockSyncListener?.({
        v: 1,
        id: 'tab-a:3',
        origin: 'tab-a',
        issuedAt: Date.now() + 1,
        type: 'scope-updated',
        scope: 'application',
        revision: '3',
        overrides: {},
      });
      resolveFirst({
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '2' },
        },
      });
    });

    await waitFor(() => expect(getAppConfigsProfile).toHaveBeenCalledTimes(2));
    act(() => {
      resolveSecond({
        theme: {
          fixedHeader: false,
          colorWeak: true,
          _meta: { v: 1, scope: 'application', revision: '3' },
        },
      });
    });

    await waitFor(() =>
      expect(mockRuntimeState.themeRuntime.layers.application).toMatchObject({
        revision: '3',
        overrides: { fixedHeader: false, colorWeak: true },
      }),
    );
  });

  it('releases a timed-out reconciliation so queued invalidation can converge', async () => {
    jest.useFakeTimers();
    (getAppConfigsProfile as jest.Mock)
      .mockReturnValueOnce(new Promise(() => {}))
      .mockResolvedValueOnce({
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '2' },
        },
      });
    render(<ThemeRuntimeBridge />);

    act(() => {
      mockSyncListener?.({
        v: 1,
        id: 'tab-a:timeout:1',
        origin: 'tab-a',
        issuedAt: Date.now(),
        type: 'scope-updated',
        scope: 'application',
        revision: '2',
        overrides: {},
      });
    });
    await act(async () => Promise.resolve());
    expect(getAppConfigsProfile).toHaveBeenCalledTimes(1);

    act(() => {
      mockSyncListener?.({
        v: 1,
        id: 'tab-a:timeout:2',
        origin: 'tab-a',
        issuedAt: Date.now() + 1,
        type: 'scope-updated',
        scope: 'application',
        revision: '2',
        overrides: {},
      });
      jest.advanceTimersByTime(THEME_RECONCILE_TIMEOUT_MS);
    });
    await act(async () => Promise.resolve());
    await act(async () => Promise.resolve());

    expect(getAppConfigsProfile).toHaveBeenCalledTimes(2);
    expect(mockRuntimeState.themeRuntime.layers.application).toMatchObject({
      revision: '2',
      overrides: { fixedHeader: true },
    });
  });

  it('actively repairs a degraded application scope when the bridge mounts', async () => {
    mockRuntimeState.themeRuntime.degradedScopes = ['application'];
    (getAppConfigsProfile as jest.Mock).mockResolvedValue({
      theme: {
        fixedHeader: true,
        _meta: { v: 1, scope: 'application', revision: '2' },
      },
    });

    render(<ThemeRuntimeBridge />);

    await waitFor(() => expect(getAppConfigsProfile).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(mockRuntimeState.themeRuntime.layers.application).toMatchObject({
        revision: '2',
        overrides: { fixedHeader: true },
      }),
    );
    expect(mockRuntimeState.themeRuntime.degradedScopes || []).not.toContain('application');
  });

  it('fails closed and re-bootstraps after another tab clears the active identity', () => {
    localStorage.setItem(THEME_AUTH_SESSION_KEY, 'session-a');
    mockRuntimeState = {
      ...mockRuntimeState,
      currentUser: { id: 'user-a', name: 'User A' },
      userConfig: {
        theme: {
          colorWeak: true,
          _meta: { v: 1, scope: 'user', revision: '4' },
        },
      },
      themeRuntime: {
        ...mockRuntimeState.themeRuntime,
        authSessionId: 'session-a',
        layers: {
          ...mockRuntimeState.themeRuntime.layers,
          user: {
            scope: 'user',
            revision: '4',
            overrides: { colorWeak: true },
            versioned: true,
          },
        },
      },
    };
    render(<ThemeRuntimeBridge />);

    act(() => {
      mockSyncListener?.({
        v: 1,
        id: 'remote-tab:identity:1',
        origin: 'remote-tab',
        issuedAt: Date.now(),
        type: 'identity-cleared',
        previousAuthSessionId: 'session-a',
      });
    });

    expect(mockRuntimeState.currentUser).toBeUndefined();
    expect(mockRuntimeState.userConfig).toBeUndefined();
    expect(mockRuntimeState.themeRuntime.authSessionId).toBeUndefined();
    expect(mockRuntimeState.themeRuntime.layers.user).toBeUndefined();
    expect(mockHistoryGo).toHaveBeenCalledWith(0);
  });

  it('discards a delayed personal profile after identity rotation and reloads the new identity', async () => {
    localStorage.setItem(THEME_AUTH_SESSION_KEY, 'session-a');
    mockRuntimeState = {
      ...mockRuntimeState,
      currentUser: { id: 'user-a' },
      userConfig: {
        theme: { _meta: { v: 1, scope: 'user', revision: '1' } },
      },
      themeRuntime: {
        ...mockRuntimeState.themeRuntime,
        authSessionId: 'session-a',
        layers: {
          ...mockRuntimeState.themeRuntime.layers,
          user: {
            scope: 'user',
            revision: '1',
            overrides: {},
            versioned: true,
          },
        },
      },
    };
    let resolveUserA!: (profile: Record<string, unknown>) => void;
    let resolveUserB!: (profile: Record<string, unknown>) => void;
    (getUserConfigsProfile as jest.Mock)
      .mockReturnValueOnce(
        new Promise<Record<string, unknown>>((resolve) => {
          resolveUserA = resolve;
        }),
      )
      .mockReturnValueOnce(
        new Promise<Record<string, unknown>>((resolve) => {
          resolveUserB = resolve;
        }),
      );
    const view = render(<ThemeRuntimeBridge />);

    act(() => {
      mockSyncListener?.({
        v: 1,
        id: 'tab-a:user:2',
        origin: 'tab-a',
        issuedAt: Date.now(),
        type: 'scope-updated',
        scope: 'user',
        authSessionId: 'session-a',
        revision: '2',
        overrides: {},
      });
    });
    await waitFor(() => expect(getUserConfigsProfile).toHaveBeenCalledTimes(1));

    localStorage.setItem(THEME_AUTH_SESSION_KEY, 'session-b');
    mockRuntimeState = {
      ...mockRuntimeState,
      currentUser: { id: 'user-b' },
      userConfig: {
        theme: { _meta: { v: 1, scope: 'user', revision: '1' } },
      },
      themeRuntime: {
        ...mockRuntimeState.themeRuntime,
        authSessionId: 'session-b',
      },
    };
    view.rerender(<ThemeRuntimeBridge />);
    act(() => {
      resolveUserA({
        theme: {
          colorWeak: true,
          _meta: { v: 1, scope: 'user', revision: '2' },
        },
      });
    });

    await waitFor(() => expect(getUserConfigsProfile).toHaveBeenCalledTimes(2));
    act(() => {
      resolveUserB({
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'user', revision: '3' },
        },
      });
    });

    await waitFor(() =>
      expect(mockRuntimeState.themeRuntime.layers.user).toMatchObject({
        revision: '3',
        overrides: { fixedHeader: true },
      }),
    );
    expect(mockRuntimeState.themeRuntime.layers.user.overrides).not.toHaveProperty('colorWeak');
  });
});
