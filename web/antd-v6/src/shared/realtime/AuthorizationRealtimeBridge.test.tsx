import { act, render, waitFor } from '@testing-library/react';
import { useModel } from '@umijs/max';
import { beforeEach, describe, expect, it, type Mock, vi } from 'vitest';
import {
  requestAuthorizationRefresh,
  requestAuthorizationRevisionRefresh,
} from '../auth/freshness';
import { redirectToLogin } from '../auth/session';
import type { InitialState } from '../auth/types';
import { queryClient, queryKeys } from '../query/client';
import AuthorizationRealtimeBridge from './AuthorizationRealtimeBridge';
import {
  type AuthorizationRealtimeSessionOptions,
  startAuthorizationRealtimeSession,
} from './socket';

vi.mock('@umijs/max', () => ({ useModel: vi.fn() }));
vi.mock('../auth/freshness', () => ({
  requestAuthorizationRefresh: vi.fn(),
  requestAuthorizationRevisionRefresh: vi.fn(),
}));
vi.mock('../auth/session', () => ({ redirectToLogin: vi.fn() }));
vi.mock('../theme/runtime', () => ({ clearUserThemeRuntime: vi.fn() }));
vi.mock('../theme/snapshot', () => ({ clearThemeIdentitySession: vi.fn() }));
vi.mock('./socket', async (importOriginal) => {
  const original = await importOriginal<typeof import('./socket')>();
  return { ...original, startAuthorizationRealtimeSession: vi.fn() };
});

const mockedUseModel = vi.mocked(useModel);
const mockedStart = vi.mocked(startAuthorizationRealtimeSession);

function initialState(): InitialState {
  return {
    currentUser: { id: 'user-1', permissions: {} },
    settings: {},
    authorizedMenu: [],
    fetchCurrentUser: vi.fn(),
  };
}

describe('AuthorizationRealtimeBridge', () => {
  let current: InitialState;
  let setInitialState: ReturnType<typeof vi.fn>;
  let options: AuthorizationRealtimeSessionOptions;
  let stop: () => void;
  let stopSpy: Mock<() => void>;

  beforeEach(() => {
    queryClient.clear();
    current = initialState();
    setInitialState = vi.fn(async (update) => {
      current = typeof update === 'function' ? update(current) : update;
    });
    mockedUseModel.mockImplementation(() => ({ initialState: current, setInitialState }) as never);
    stopSpy = vi.fn<() => void>();
    stop = () => {
      stopSpy();
    };
    mockedStart.mockImplementation((value) => {
      options = value;
      return stop;
    });
  });

  it('bridges revision, reconnect, and terminal session events', async () => {
    queryClient.setQueryData(queryKeys.workplace, { confidential: true });
    const view = render(<AuthorizationRealtimeBridge />);
    expect(mockedStart).toHaveBeenCalledOnce();

    act(() => options.onAuthorizationRevision('8'));
    expect(requestAuthorizationRevisionRefresh).toHaveBeenCalledWith('8');
    act(() => options.onReconnected?.());
    expect(requestAuthorizationRefresh).toHaveBeenCalledOnce();
    expect(options.shouldReconnect?.({ response: { status: 401 } })).toBe(false);
    expect(options.shouldReconnect?.({ response: { status: 503 } })).toBe(true);

    act(() => options.onKick('revoked'));
    await waitFor(() => expect(redirectToLogin).toHaveBeenCalledOnce());
    expect(queryClient.getQueryData(queryKeys.workplace)).toBeUndefined();
    expect(current.currentUser).toBeUndefined();

    view.unmount();
    expect(stopSpy).toHaveBeenCalledOnce();
  });
});
