import { act, render, waitFor } from '@testing-library/react';
import { useModel } from '@umijs/max';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { queryClient, queryKeys } from '../query/client';
import AuthorizationFreshnessBridge from './AuthorizationFreshnessBridge';
import { fetchAuthorizedMenu } from './authorization';
import { AUTHORIZATION_REFRESH_EVENT } from './freshness';
import type { InitialState } from './types';

vi.mock('@umijs/max', () => ({ useModel: vi.fn() }));
vi.mock('./authorization', () => ({ fetchAuthorizedMenu: vi.fn() }));

const mockedUseModel = vi.mocked(useModel);
const mockedFetchAuthorizedMenu = vi.mocked(fetchAuthorizedMenu);

function state(permissions: Record<string, boolean>): InitialState {
  return {
    currentUser: {
      id: 'user-1',
      roleID: 'role-1',
      role: { id: 'role-1', root: false },
      permissions,
    },
    settings: {},
    authorizedMenu: [{ key: 'workplace', path: '/workplace', permission: '/welcome' }],
    fetchCurrentUser: vi.fn(),
  };
}

describe('AuthorizationFreshnessBridge', () => {
  let current: InitialState;
  let setInitialState: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    queryClient.clear();
    mockedFetchAuthorizedMenu.mockReset();
    current = state({ '/welcome': true });
    setInitialState = vi.fn(async (update) => {
      current = typeof update === 'function' ? update(current) : update;
    });
    mockedUseModel.mockImplementation(() => ({ initialState: current, setInitialState }) as never);
  });

  it('replaces changed authorization and evicts non-bootstrap query data', async () => {
    const existingUser = current.currentUser;
    if (!existingUser) throw new Error('test identity is missing');
    const refreshed = {
      ...existingUser,
      permissions: { '/app-config': true },
    };
    current.fetchCurrentUser = vi.fn().mockResolvedValue(refreshed);
    mockedFetchAuthorizedMenu.mockResolvedValue([
      { key: 'app-config', path: '/app-config', permission: '/app-config' },
    ]);
    queryClient.setQueryData(queryKeys.accountTokens('user-1'), [{ id: 'sensitive' }]);
    queryClient.setQueryData(queryKeys.applicationProfile, { public: true });

    render(<AuthorizationFreshnessBridge />);
    act(() => window.dispatchEvent(new Event(AUTHORIZATION_REFRESH_EVENT)));

    await waitFor(() => expect(setInitialState).toHaveBeenCalled());
    expect(current.currentUser?.permissions).toEqual({ '/app-config': true });
    expect(current.authorizedMenu).toEqual([
      { key: 'app-config', path: '/app-config', permission: '/app-config' },
    ]);
    expect(current.authorizationVersion).toBe(1);
    expect(queryClient.getQueryData(queryKeys.accountTokens('user-1'))).toBeUndefined();
    expect(queryClient.getQueryData(queryKeys.applicationProfile)).toEqual({ public: true });
  });

  it('fails closed and evicts domain data when the menu cannot be confirmed', async () => {
    current.fetchCurrentUser = vi.fn().mockResolvedValue(current.currentUser);
    mockedFetchAuthorizedMenu.mockRejectedValue(
      Object.assign(new Error('unavailable'), { response: { status: 503 } }),
    );
    queryClient.setQueryData(queryKeys.workplace, { confidential: true });

    render(<AuthorizationFreshnessBridge />);
    act(() => window.dispatchEvent(new Event(AUTHORIZATION_REFRESH_EVENT)));

    await waitFor(() =>
      expect(current.startupFailure).toEqual({ area: 'authorization', status: 503 }),
    );
    expect(current.authorizedMenu).toEqual([]);
    expect(current.authorizationVersion).toBe(1);
    expect(queryClient.getQueryData(queryKeys.workplace)).toBeUndefined();
  });

  it('throttles passive focus refreshes', async () => {
    const now = vi.spyOn(Date, 'now').mockReturnValue(1_000);
    current.fetchCurrentUser = vi.fn().mockResolvedValue(current.currentUser);
    mockedFetchAuthorizedMenu.mockResolvedValue(current.authorizedMenu);

    render(<AuthorizationFreshnessBridge />);
    act(() => window.dispatchEvent(new Event('focus')));
    expect(current.fetchCurrentUser).not.toHaveBeenCalled();

    now.mockReturnValue(31_000);
    act(() => window.dispatchEvent(new Event('focus')));
    await waitFor(() => expect(current.fetchCurrentUser).toHaveBeenCalledTimes(1));
    now.mockRestore();
  });
});
