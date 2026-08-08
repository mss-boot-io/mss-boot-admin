import { act, render, waitFor } from '@testing-library/react';
import { useModel } from '@umijs/max';
import { getAuthToken } from '@/utils/authStorage';
import { PERMISSION_REFRESH_EVENT } from '@/utils/permissionFreshness';
import PermissionFreshnessBridge from './PermissionFreshnessBridge';

const React = require('react');

jest.mock('@umijs/max', () => ({ useModel: jest.fn() }));
jest.mock('@/utils/authStorage', () => ({ getAuthToken: jest.fn() }));

describe('PermissionFreshnessBridge', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (getAuthToken as jest.Mock).mockReturnValue('token');
  });

  it('refreshes identity and invalidates the runtime menu after an explicit event', async () => {
    const currentUser = { id: 'user-1', permissions: { '/role': true } };
    const fetchUserInfo = jest.fn().mockResolvedValue(currentUser);
    const setInitialState = jest.fn();
    (useModel as jest.Mock).mockReturnValue({
      initialState: { fetchUserInfo, permissionRefreshVersion: 2 },
      setInitialState,
    });
    render(<PermissionFreshnessBridge />);

    act(() => window.dispatchEvent(new Event(PERMISSION_REFRESH_EVENT)));

    await waitFor(() => expect(fetchUserInfo).toHaveBeenCalledTimes(1));
    const update = setInitialState.mock.calls[0][0];
    expect(update({ permissionRefreshVersion: 2 })).toMatchObject({
      currentUser,
      permissionRefreshVersion: 3,
    });
  });

  it('clears a stale rendered identity after the shared 401 path removes the token', async () => {
    const fetchUserInfo = jest.fn().mockResolvedValue(undefined);
    const setInitialState = jest.fn();
    (getAuthToken as jest.Mock).mockReturnValue(undefined);
    (useModel as jest.Mock).mockReturnValue({ initialState: { fetchUserInfo }, setInitialState });
    render(<PermissionFreshnessBridge />);

    act(() => window.dispatchEvent(new Event(PERMISSION_REFRESH_EVENT)));

    await waitFor(() => expect(setInitialState).toHaveBeenCalledTimes(1));
    expect(setInitialState.mock.calls[0][0]({ currentUser: { id: 'old' } })).toMatchObject({
      currentUser: undefined,
      permissionRefreshVersion: 1,
    });
  });

  it('queues a mutation refresh behind an already in-flight refresh', async () => {
    let resolveFirst: ((value: API.User) => void) | undefined;
    const fetchUserInfo = jest
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise<API.User>((resolve) => {
            resolveFirst = resolve;
          }),
      )
      .mockResolvedValue({ id: 'user-1', permissions: { '/menu': true } });
    const setInitialState = jest.fn();
    (useModel as jest.Mock).mockReturnValue({ initialState: { fetchUserInfo }, setInitialState });
    render(<PermissionFreshnessBridge />);

    act(() => {
      window.dispatchEvent(new Event(PERMISSION_REFRESH_EVENT));
      window.dispatchEvent(new Event(PERMISSION_REFRESH_EVENT));
    });
    expect(fetchUserInfo).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolveFirst?.({ id: 'user-1', permissions: { '/menu': false } });
      await Promise.resolve();
    });

    await waitFor(() => expect(fetchUserInfo).toHaveBeenCalledTimes(2));
  });

  it('contains rejected refresh requests and retains the last identity while the token survives', async () => {
    const fetchUserInfo = jest.fn().mockRejectedValue(new Error('network unavailable'));
    const setInitialState = jest.fn();
    (useModel as jest.Mock).mockReturnValue({ initialState: { fetchUserInfo }, setInitialState });
    render(<PermissionFreshnessBridge />);

    act(() => window.dispatchEvent(new Event(PERMISSION_REFRESH_EVENT)));

    await waitFor(() => expect(fetchUserInfo).toHaveBeenCalledTimes(1));
    await act(async () => Promise.resolve());
    expect(setInitialState).not.toHaveBeenCalled();
  });

  it('throttles passive refreshes when the visible window regains focus', async () => {
    const now = jest.spyOn(Date, 'now').mockReturnValue(1_000);
    const fetchUserInfo = jest.fn().mockResolvedValue({ id: 'user-1' });
    const setInitialState = jest.fn();
    (useModel as jest.Mock).mockReturnValue({ initialState: { fetchUserInfo }, setInitialState });
    render(<PermissionFreshnessBridge />);

    act(() => window.dispatchEvent(new Event('focus')));
    expect(fetchUserInfo).not.toHaveBeenCalled();

    now.mockReturnValue(31_000);
    act(() => window.dispatchEvent(new Event('focus')));
    await waitFor(() => expect(fetchUserInfo).toHaveBeenCalledTimes(1));

    act(() => window.dispatchEvent(new Event('focus')));
    expect(fetchUserInfo).toHaveBeenCalledTimes(1);
    now.mockRestore();
  });
});
