import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import * as React from 'react';
import Theme from './theme';
import { ThemeRevisionConflictError } from '@/services/admin/themeSettings';
import { rotateThemeAuthSession, THEME_AUTH_SESSION_KEY } from '@/utils/themeSession';

let mockRuntimeState: any;
const mockSetInitialState = jest.fn((updater) => {
  mockRuntimeState = updater(mockRuntimeState);
});

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: ({ defaultMessage, id }: { defaultMessage?: string; id: string }) =>
      defaultMessage || id,
  }),
  useModel: () => ({ initialState: mockRuntimeState, setInitialState: mockSetInitialState }),
}));

jest.mock('antd', () => {
  const actual = jest.requireActual('antd');
  return {
    ...actual,
    message: { useMessage: () => [{ success: jest.fn() }, null] },
  };
});

describe('shared theme editor', () => {
  const storageValues = new Map<string, string>();

  beforeEach(() => {
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
    localStorage.setItem(THEME_AUTH_SESSION_KEY, 'test-session');
    mockRuntimeState = {
      currentUser: { role: { root: false }, permissions: {} },
      appConfig: { theme: { navTheme: 'light', fixedHeader: false } },
      userConfig: { theme: { navTheme: 'realDark' } },
      settings: {},
      themeRuntime: {
        schemaVersion: 1,
        authSessionId: 'test-session',
        layers: {},
      },
    };
    mockSetInitialState.mockClear();
  });

  it('restores a personal override through the injected user adapter and applies immediately', async () => {
    const adapter = {
      scope: 'user' as const,
      load: jest.fn().mockResolvedValue({
        scope: 'user',
        revision: '1',
        overrides: { navTheme: 'realDark' },
        versioned: true,
      }),
      patch: jest.fn(),
      reset: jest.fn().mockResolvedValue({
        scope: 'user',
        revision: '2',
        overrides: {},
        versioned: true,
      }),
    };
    const onApplied = jest.fn();
    render(<Theme adapter={adapter} onApplied={onApplied} scope="user" />);

    const reset = await screen.findByRole('button', { name: 'Restore inherited' });
    fireEvent.click(reset);

    await waitFor(() =>
      expect(adapter.reset).toHaveBeenCalledWith(
        ['navTheme'],
        expect.objectContaining({ revision: '1' }),
      ),
    );
    expect(onApplied).toHaveBeenCalledWith(
      expect.objectContaining({ navTheme: 'light', fixedHeader: false }),
      {},
    );
    expect(mockRuntimeState.userConfig.theme).toEqual({
      _meta: { v: 1, scope: 'user', revision: '2' },
    });
    expect(mockRuntimeState.settings).toEqual(expect.objectContaining({ navTheme: 'light' }));
  });

  it('requires confirmation before resetting every override', async () => {
    const adapter = {
      scope: 'user' as const,
      load: jest.fn().mockResolvedValue({
        scope: 'user',
        revision: '1',
        overrides: { navTheme: 'realDark' },
        versioned: true,
      }),
      patch: jest.fn(),
      reset: jest.fn().mockResolvedValue({
        scope: 'user',
        revision: '2',
        overrides: {},
        versioned: true,
      }),
    };
    render(<Theme adapter={adapter} scope="user" />);

    const resetAll = await screen.findByRole('button', {
      name: 'Restore all inherited settings',
    });
    fireEvent.click(resetAll);
    expect(adapter.reset).not.toHaveBeenCalled();

    fireEvent.click(await screen.findByRole('button', { name: 'OK' }));
    await waitFor(() =>
      expect(adapter.reset).toHaveBeenCalledWith(
        undefined,
        expect.objectContaining({ revision: '1' }),
      ),
    );
  });

  it('renders application settings as read-only without the write permission', async () => {
    mockRuntimeState.currentUser = { role: { root: false }, permissions: {} };
    const adapter = {
      scope: 'application' as const,
      load: jest.fn().mockResolvedValue({
        scope: 'application',
        revision: '1',
        overrides: { navTheme: 'light' },
        versioned: true,
      }),
      patch: jest.fn(),
      reset: jest.fn(),
    };

    render(<Theme adapter={adapter} scope="application" />);

    expect(
      await screen.findByText(
        'You can view the application theme but need configuration write permission to change it.',
      ),
    ).toBeTruthy();
    expect((screen.getByRole('button', { name: 'Save theme' }) as HTMLButtonElement).disabled).toBe(
      true,
    );
    expect(adapter.patch).not.toHaveBeenCalled();
  });

  it('allows a non-root user with the application control component permission to edit', async () => {
    mockRuntimeState.currentUser = {
      role: { root: false },
      permissions: { '/app-config/control': 'component' },
    };
    const adapter = {
      scope: 'application' as const,
      load: jest.fn().mockResolvedValue({
        scope: 'application',
        revision: '1',
        overrides: {},
        versioned: true,
      }),
      patch: jest.fn().mockResolvedValue({
        scope: 'application',
        revision: '2',
        overrides: { fixedHeader: true },
        versioned: true,
      }),
      reset: jest.fn(),
    };

    render(<Theme adapter={adapter} scope="application" />);

    const fixedHeader = (await screen.findAllByRole('switch'))[0] as HTMLButtonElement;
    expect(fixedHeader.disabled).toBe(false);
    fireEvent.click(fixedHeader);
    fireEvent.click(screen.getByRole('button', { name: 'Save theme' }));

    await waitFor(() => expect(adapter.patch).toHaveBeenCalledTimes(1));
  });

  it('blocks blind writes after an initial load failure and enables legacy compatibility after retry', async () => {
    mockRuntimeState.currentUser = { role: { root: true }, permissions: {} };
    const legacyResource = {
      scope: 'application' as const,
      revision: '0',
      overrides: {},
      versioned: false,
    };
    const adapter = {
      scope: 'application' as const,
      load: jest
        .fn()
        .mockRejectedValueOnce(new Error('profile unavailable'))
        .mockResolvedValueOnce(legacyResource),
      patch: jest.fn().mockResolvedValue({
        ...legacyResource,
        overrides: { fixedHeader: true },
      }),
      reset: jest.fn(),
    };

    render(<Theme adapter={adapter} scope="application" />);

    expect(await screen.findByText('Unable to update theme settings')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Save theme' })).toBeNull();
    expect(screen.queryAllByRole('switch')).toHaveLength(0);
    expect(adapter.patch).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    const save = await screen.findByRole('button', { name: 'Save theme' });
    const fixedHeader = (await screen.findAllByRole('switch'))[0] as HTMLButtonElement;
    expect(fixedHeader.disabled).toBe(false);
    fireEvent.click(fixedHeader);
    fireEvent.click(save);

    await waitFor(() =>
      expect(adapter.patch).toHaveBeenCalledWith(
        { fixedHeader: true },
        expect.objectContaining({ revision: '0', versioned: false }),
      ),
    );
  });

  it('renders an explicit permission-denied state for an application theme read forbidden by the backend', async () => {
    mockRuntimeState.currentUser = { role: { root: false }, permissions: {} };
    const forbidden = Object.assign(new Error('Forbidden'), {
      response: { status: 403 },
    });
    const adapter = {
      scope: 'application' as const,
      load: jest.fn().mockRejectedValue(forbidden),
      patch: jest.fn(),
      reset: jest.fn(),
    };

    render(<Theme adapter={adapter} scope="application" />);

    expect(
      await screen.findByText('You do not have permission to view the application theme.'),
    ).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Save theme' })).toBeNull();
    expect(adapter.patch).not.toHaveBeenCalled();
  });

  it('enables save only after a personal field becomes dirty', async () => {
    const adapter = {
      scope: 'user' as const,
      load: jest.fn().mockResolvedValue({
        scope: 'user',
        revision: '1',
        overrides: {},
        versioned: true,
      }),
      patch: jest.fn().mockResolvedValue({
        scope: 'user',
        revision: '2',
        overrides: { fixedHeader: true },
        versioned: true,
      }),
      reset: jest.fn(),
    };
    render(<Theme adapter={adapter} scope="user" />);

    const save = await screen.findByRole('button', { name: 'Save theme' });
    expect((save as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click((await screen.findAllByRole('switch'))[0]);
    expect((save as HTMLButtonElement).disabled).toBe(false);
    fireEvent.click(save);

    await waitFor(() =>
      expect(adapter.patch).toHaveBeenCalledWith(
        { fixedHeader: true },
        expect.objectContaining({ revision: '1' }),
      ),
    );
    expect((save as HTMLButtonElement).disabled).toBe(true);
  });

  it('blocks edits and duplicate mutations while a save is in flight', async () => {
    let resolvePatch!: (resource: {
      scope: 'user';
      revision: string;
      overrides: { fixedHeader: boolean };
      versioned: boolean;
    }) => void;
    const adapter = {
      scope: 'user' as const,
      load: jest.fn().mockResolvedValue({
        scope: 'user',
        revision: '1',
        overrides: {},
        versioned: true,
      }),
      patch: jest.fn().mockReturnValue(
        new Promise<Parameters<typeof resolvePatch>[0]>((resolve) => {
          resolvePatch = resolve;
        }),
      ),
      reset: jest.fn(),
    };
    render(<Theme adapter={adapter} scope="user" />);

    const switches = await screen.findAllByRole('switch');
    const fixedHeader = switches[0] as HTMLButtonElement;
    const colorWeak = switches[2] as HTMLButtonElement;
    const save = screen.getByRole('button', { name: 'Save theme' }) as HTMLButtonElement;
    fireEvent.click(fixedHeader);
    fireEvent.click(save);
    await waitFor(() => expect(adapter.patch).toHaveBeenCalledTimes(1));

    expect(fixedHeader.disabled).toBe(true);
    expect(colorWeak.disabled).toBe(true);
    expect(save.disabled).toBe(true);
    fireEvent.click(colorWeak);
    fireEvent.click(save);
    expect(adapter.patch).toHaveBeenCalledTimes(1);
    expect(colorWeak.getAttribute('aria-checked')).toBe('false');

    resolvePatch({
      scope: 'user',
      revision: '2',
      overrides: { fixedHeader: true },
      versioned: true,
    });
    await waitFor(() => expect(fixedHeader.disabled).toBe(false));
    expect(save.disabled).toBe(true);
  });

  it('preserves an application draft when another tab publishes a newer runtime resource', async () => {
    mockRuntimeState.currentUser = { role: { root: true }, permissions: {} };
    const initialResource = {
      scope: 'application' as const,
      revision: '1',
      overrides: {},
      versioned: true,
    };
    const adapter = {
      scope: 'application' as const,
      load: jest.fn().mockResolvedValue(initialResource),
      patch: jest.fn(),
      reset: jest.fn(),
    };
    const view = render(<Theme adapter={adapter} scope="application" />);

    const switches = await screen.findAllByRole('switch');
    const colorWeak = switches[2] as HTMLButtonElement;
    fireEvent.click(colorWeak);
    expect(colorWeak.getAttribute('aria-checked')).toBe('true');
    expect((screen.getByRole('button', { name: 'Save theme' }) as HTMLButtonElement).disabled).toBe(
      false,
    );

    const newerResource = {
      scope: 'application' as const,
      revision: '2',
      overrides: { fixedHeader: true },
      versioned: true,
    };
    mockRuntimeState = {
      ...mockRuntimeState,
      appConfig: {
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '2' },
        },
      },
      themeRuntime: {
        ...mockRuntimeState.themeRuntime,
        layers: { application: newerResource },
      },
    };
    view.rerender(<Theme adapter={adapter} scope="application" />);

    expect(await screen.findByText('Theme settings changed in another tab')).toBeTruthy();
    expect(colorWeak.getAttribute('aria-checked')).toBe('true');
    expect(adapter.load).toHaveBeenCalledTimes(1);
    expect((screen.getByRole('button', { name: 'Save theme' }) as HTMLButtonElement).disabled).toBe(
      true,
    );
  });

  it('retries a stale initial read before enabling mutations', async () => {
    mockRuntimeState.currentUser = { role: { root: true }, permissions: {} };
    let resolveFirst!: (resource: {
      scope: 'application';
      revision: string;
      overrides: Record<string, never>;
      versioned: boolean;
    }) => void;
    const adapter = {
      scope: 'application' as const,
      load: jest
        .fn()
        .mockReturnValueOnce(
          new Promise((resolve) => {
            resolveFirst = resolve;
          }),
        )
        .mockResolvedValueOnce({
          scope: 'application',
          revision: '2',
          overrides: { fixedHeader: true },
          versioned: true,
        }),
      patch: jest.fn(),
      reset: jest.fn(),
    };
    const view = render(<Theme adapter={adapter} scope="application" />);
    await waitFor(() => expect(adapter.load).toHaveBeenCalledTimes(1));

    const newerResource = {
      scope: 'application' as const,
      revision: '2',
      overrides: { fixedHeader: true },
      versioned: true,
    };
    mockRuntimeState = {
      ...mockRuntimeState,
      appConfig: {
        theme: {
          fixedHeader: true,
          _meta: { v: 1, scope: 'application', revision: '2' },
        },
      },
      themeRuntime: {
        ...mockRuntimeState.themeRuntime,
        layers: { application: newerResource },
      },
    };
    view.rerender(<Theme adapter={adapter} scope="application" />);
    resolveFirst({ scope: 'application', revision: '1', overrides: {}, versioned: true });

    await waitFor(() => expect(adapter.load).toHaveBeenCalledTimes(2));
    const save = await screen.findByRole('button', { name: 'Save theme' });
    fireEvent.click((await screen.findAllByRole('switch'))[2]);
    expect((save as HTMLButtonElement).disabled).toBe(false);
  });

  it('preserves a touched draft on 412 and retries only after explicit review', async () => {
    const current = {
      scope: 'user' as const,
      revision: '2',
      overrides: { fixedHeader: false },
      versioned: true,
    };
    const adapter = {
      scope: 'user' as const,
      load: jest.fn().mockResolvedValue({
        scope: 'user',
        revision: '1',
        overrides: {},
        versioned: true,
      }),
      patch: jest
        .fn()
        .mockRejectedValueOnce(new ThemeRevisionConflictError(current))
        .mockResolvedValueOnce({
          scope: 'user',
          revision: '3',
          overrides: { fixedHeader: true },
          versioned: true,
        }),
      reset: jest.fn(),
    };
    render(<Theme adapter={adapter} scope="user" />);

    const fixedHeader = (await screen.findAllByRole('switch'))[0] as HTMLButtonElement;
    fireEvent.click(fixedHeader);
    const save = screen.getByRole('button', { name: 'Save theme' }) as HTMLButtonElement;
    fireEvent.click(save);

    expect(await screen.findByText('Theme settings changed in another tab')).toBeTruthy();
    expect(fixedHeader.getAttribute('aria-checked')).toBe('true');
    expect(save.disabled).toBe(true);

    fireEvent.click(screen.getByRole('button', { name: 'Review and retry' }));
    expect(save.disabled).toBe(false);
    fireEvent.click(save);

    await waitFor(() => expect(adapter.patch).toHaveBeenCalledTimes(2));
    expect(adapter.patch).toHaveBeenLastCalledWith(
      { fixedHeader: true },
      expect.objectContaining({ revision: '2' }),
    );
  });

  it('refuses personal writes when browser credentials moved to another auth session', async () => {
    localStorage.setItem(THEME_AUTH_SESSION_KEY, 'session-b');
    mockRuntimeState.themeRuntime.authSessionId = 'session-a';
    const adapter = {
      scope: 'user' as const,
      load: jest.fn().mockResolvedValue({
        scope: 'user',
        revision: '4',
        overrides: {},
        versioned: true,
      }),
      patch: jest.fn(),
      reset: jest.fn(),
    };

    render(<Theme adapter={adapter} scope="user" />);

    expect(
      await screen.findByText(
        'The signed-in identity changed in another tab. Reload or sign in again before editing theme settings.',
      ),
    ).toBeTruthy();
    expect((screen.getByRole('button', { name: 'Save theme' }) as HTMLButtonElement).disabled).toBe(
      true,
    );
    expect(adapter.load).not.toHaveBeenCalled();
    expect(adapter.patch).not.toHaveBeenCalled();
  });

  it('fails closed when the auth session rotates after a personal draft is rendered', async () => {
    const adapter = {
      scope: 'user' as const,
      load: jest.fn().mockResolvedValue({
        scope: 'user',
        revision: '4',
        overrides: {},
        versioned: true,
      }),
      patch: jest.fn(),
      reset: jest.fn(),
    };
    render(<Theme adapter={adapter} scope="user" />);

    fireEvent.click((await screen.findAllByRole('switch'))[0]);
    expect((screen.getByRole('button', { name: 'Save theme' }) as HTMLButtonElement).disabled).toBe(
      false,
    );
    rotateThemeAuthSession({ persistent: true });
    fireEvent.click(screen.getByRole('button', { name: 'Save theme' }));

    expect(
      await screen.findByText(
        'The signed-in identity changed in another tab. Reload or sign in again before editing theme settings.',
      ),
    ).toBeTruthy();
    expect(adapter.patch).not.toHaveBeenCalled();
    expect((screen.getByRole('button', { name: 'Save theme' }) as HTMLButtonElement).disabled).toBe(
      true,
    );
  });

  it('fails closed when the auth session rotates after an application draft is rendered', async () => {
    mockRuntimeState.currentUser = { role: { root: true }, permissions: {} };
    const adapter = {
      scope: 'application' as const,
      load: jest.fn().mockResolvedValue({
        scope: 'application',
        revision: '4',
        overrides: {},
        versioned: true,
      }),
      patch: jest.fn(),
      reset: jest.fn(),
    };
    render(<Theme adapter={adapter} scope="application" />);

    fireEvent.click((await screen.findAllByRole('switch'))[0]);
    rotateThemeAuthSession({ persistent: true });
    fireEvent.click(screen.getByRole('button', { name: 'Save theme' }));

    expect(
      await screen.findByText(
        'The signed-in identity changed in another tab. Reload or sign in again before editing theme settings.',
      ),
    ).toBeTruthy();
    expect(adapter.patch).not.toHaveBeenCalled();
  });

  it('discards a delayed personal save response after the auth session rotates', async () => {
    let resolvePatch!: (resource: {
      scope: 'user';
      revision: string;
      overrides: { fixedHeader: boolean };
      versioned: boolean;
    }) => void;
    const patchResponse = new Promise<Parameters<typeof resolvePatch>[0]>((resolve) => {
      resolvePatch = resolve;
    });
    const adapter = {
      scope: 'user' as const,
      load: jest.fn().mockResolvedValue({
        scope: 'user',
        revision: '1',
        overrides: {},
        versioned: true,
      }),
      patch: jest.fn().mockReturnValue(patchResponse),
      reset: jest.fn(),
    };
    render(<Theme adapter={adapter} scope="user" />);

    fireEvent.click((await screen.findAllByRole('switch'))[0]);
    fireEvent.click(screen.getByRole('button', { name: 'Save theme' }));
    await waitFor(() => expect(adapter.patch).toHaveBeenCalledTimes(1));

    const replacementSession = rotateThemeAuthSession({ persistent: true });
    expect(replacementSession).not.toBe('test-session');
    resolvePatch({
      scope: 'user',
      revision: '2',
      overrides: { fixedHeader: true },
      versioned: true,
    });

    expect(
      await screen.findByText(
        'The signed-in identity changed in another tab. Reload or sign in again before editing theme settings.',
      ),
    ).toBeTruthy();
    expect(mockRuntimeState.userConfig.theme._meta.revision).toBe('1');
  });
});
