import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ColorPickerProps } from 'antd';
import { App } from 'antd';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { InitialState } from '@/shared/auth/types';
import { ThemeRevisionConflictError } from '@/shared/theme/api';
import type { ThemeScopeResource } from '@/shared/theme/contract';
import { replaceThemeRuntime } from '@/shared/theme/runtime';
import ThemeSettingsEditor from './ThemeSettingsEditor';

// Rendering the complete token-aware Ant Design form can exceed Vitest's 5s
// default on a busy repository verifier. Keep a local, bounded CI allowance
// without weakening any interaction assertions.
const INTERACTION_TEST_TIMEOUT_MS = 15_000;

const model = vi.hoisted(() => ({
  state: undefined as unknown,
  setInitialState: vi.fn(),
}));

const api = vi.hoisted(() => ({
  load: vi.fn(),
  patch: vi.fn(),
  reset: vi.fn(),
}));

const browserDerivatives = vi.hoisted(() => ({
  publish: vi.fn(),
  write: vi.fn(),
}));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: ({ id }: { id: string }, values?: Record<string, unknown>) =>
      values?.revision ? `${id}:${String(values.revision)}` : id,
  }),
  useModel: () => ({ initialState: model.state, setInitialState: model.setInitialState }),
}));

vi.mock('antd', async (importOriginal) => {
  const actual = await importOriginal<typeof import('antd')>();
  return {
    ...actual,
    ColorPicker: ({ onChange }: Pick<ColorPickerProps, 'onChange'>) => (
      <button
        type="button"
        aria-label="test-color-picker"
        onClick={() =>
          onChange?.(
            {
              toHexString: () => '#A1B2C3',
            } as Parameters<NonNullable<ColorPickerProps['onChange']>>[0],
            'rgb(161, 178, 195)',
          )
        }
      >
        color
      </button>
    ),
  };
});

vi.mock('@/shared/theme/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/shared/theme/api')>();
  return {
    ...actual,
    loadThemeResource: api.load,
    patchThemeResource: api.patch,
    resetThemeResource: api.reset,
  };
});

vi.mock('@/shared/theme/snapshot', () => ({ writeThemeSnapshot: browserDerivatives.write }));
vi.mock('@/shared/theme/sync', () => ({ publishThemeScopeResource: browserDerivatives.publish }));

function initialState(permissions: Record<string, boolean>): InitialState {
  return {
    currentUser: {
      id: 'user-1',
      role: { root: false },
      permissions,
    },
    authSessionId: 'session-1',
    settings: {},
    authorizedMenu: [],
    fetchCurrentUser: async () => undefined,
  };
}

function resource(
  scope: 'application' | 'user',
  revision: string,
  overrides: ThemeScopeResource['overrides'] = {},
): ThemeScopeResource {
  return { scope, revision, overrides };
}

function renderEditor(scope: 'application' | 'user') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const view = render(
    <App>
      <QueryClientProvider client={client}>
        <ThemeSettingsEditor scope={scope} />
      </QueryClientProvider>
    </App>,
  );
  return { ...view, client };
}

describe('scoped v6 theme editor', () => {
  beforeEach(() => {
    replaceThemeRuntime({});
    model.state = initialState({});
    model.setInitialState.mockReset();
    model.setInitialState.mockImplementation(
      async (
        update:
          | InitialState
          | undefined
          | ((previous: InitialState | undefined) => InitialState | undefined),
      ) => {
        model.state = typeof update === 'function' ? update(model.state as InitialState) : update;
      },
    );
    api.load.mockReset();
    api.patch.mockReset();
    api.reset.mockReset();
    browserDerivatives.publish.mockReset();
    browserDerivatives.write.mockReset();
    browserDerivatives.write.mockResolvedValue(true);
  });

  it(
    'keeps application theme read-only without the control permission',
    async () => {
      model.state = initialState({ '/app-config': true });
      api.load.mockResolvedValue(resource('application', '4', { layout: 'top' }));

      renderEditor('application');

      expect(await screen.findByText('theme.scope.application')).toBeTruthy();
      expect(api.load).toHaveBeenCalledWith('application');
      expect(screen.getByText('theme.readOnly')).toBeTruthy();
      expect(
        (screen.getByRole('button', { name: /actions\.save/ }) as HTMLButtonElement).disabled,
      ).toBe(true);
      expect(api.patch).not.toHaveBeenCalled();
    },
    INTERACTION_TEST_TIMEOUT_MS,
  );

  it(
    'saves only the touched personal field through the personal scope',
    async () => {
      model.state = initialState({});
      const base = resource('user', '7', { fixedHeader: false });
      const saved = resource('user', '8', { fixedHeader: true });
      api.load.mockResolvedValue(base);
      api.patch.mockResolvedValue(saved);

      renderEditor('user');

      await screen.findByText('theme.scope.user');
      const fixedHeaderSwitch = screen.getAllByRole('switch')[0];
      if (!fixedHeaderSwitch) throw new Error('Fixed-header switch was not rendered');
      fireEvent.click(fixedHeaderSwitch);
      fireEvent.click(screen.getByRole('button', { name: /actions\.save/ }));

      await waitFor(() =>
        expect(api.patch).toHaveBeenCalledWith('user', { fixedHeader: true }, base),
      );
      await waitFor(() => expect(screen.getByText('theme.revision:8')).toBeTruthy());
      expect(browserDerivatives.publish).toHaveBeenCalledWith(saved, 'session-1');
      expect(browserDerivatives.write).toHaveBeenCalledWith(saved, 'session-1');
    },
    INTERACTION_TEST_TIMEOUT_MS,
  );

  it(
    'saves a ColorPicker selection as a canonical six-digit hex value',
    async () => {
      model.state = initialState({});
      const base = resource('user', '9', { colorPrimary: '#1677ff' });
      const saved = resource('user', '10', { colorPrimary: '#a1b2c3' });
      api.load.mockResolvedValue(base);
      api.patch.mockResolvedValue(saved);

      renderEditor('user');

      await screen.findByText('theme.scope.user');
      fireEvent.click(screen.getByRole('button', { name: 'test-color-picker' }));
      fireEvent.click(screen.getByRole('button', { name: /actions\.save/ }));

      await waitFor(() =>
        expect(api.patch).toHaveBeenCalledWith('user', { colorPrimary: '#a1b2c3' }, base),
      );
    },
    INTERACTION_TEST_TIMEOUT_MS,
  );

  it(
    'shows an explicit 412 decision and never retries a stale draft automatically',
    async () => {
      model.state = initialState({});
      const base = resource('user', '7', { fixedHeader: false });
      const current = resource('user', '8', { fixedHeader: false, layout: 'top' });
      api.load.mockResolvedValue(base);
      api.patch.mockRejectedValue(new ThemeRevisionConflictError(current));

      renderEditor('user');

      await screen.findByText('theme.scope.user');
      const fixedHeaderSwitch = screen.getAllByRole('switch')[0];
      if (!fixedHeaderSwitch) throw new Error('Fixed-header switch was not rendered');
      fireEvent.click(fixedHeaderSwitch);
      fireEvent.click(screen.getByRole('button', { name: /actions\.save/ }));

      expect(await screen.findByText('theme.conflict.title')).toBeTruthy();
      expect(screen.getByText('theme.conflict.description:8')).toBeTruthy();
      expect(api.patch).toHaveBeenCalledTimes(1);
      expect(
        (screen.getByRole('button', { name: /actions\.save/ }) as HTMLButtonElement).disabled,
      ).toBe(true);
      expect(screen.getByRole('button', { name: 'theme.conflict.keep' })).toBeTruthy();
    },
    INTERACTION_TEST_TIMEOUT_MS,
  );
});
