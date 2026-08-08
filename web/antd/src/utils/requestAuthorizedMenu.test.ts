import { getMenuAuthorize } from '@/services/admin/menu';
import {
  clearAuthorizedMenuRequestCache,
  requestAuthorizedMenu,
} from './requestAuthorizedMenu';

jest.mock('@/services/admin/menu', () => ({
  getMenuAuthorize: jest.fn(),
}));

const mockedGetMenuAuthorize = getMenuAuthorize as jest.Mock;

describe('requestAuthorizedMenu cache', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    clearAuthorizedMenuRequestCache();
  });

  it('shares the exact in-flight and resolved promise for one identity and permission version', async () => {
    const identity = { role: { root: false }, permissions: { '/users': true } };
    let resolveMenu: ((menu: API.Menu[]) => void) | undefined;
    mockedGetMenuAuthorize.mockReturnValue(
      new Promise<API.Menu[]>((resolve) => {
        resolveMenu = resolve;
      }),
    );

    const first = requestAuthorizedMenu(identity, 7);
    const second = requestAuthorizedMenu(identity, 7);
    expect(second).toBe(first);
    expect(mockedGetMenuAuthorize).toHaveBeenCalledTimes(1);

    resolveMenu?.([{ path: '/users', type: 'MENU' }]);
    await expect(first).resolves.toEqual([{ path: '/users', type: 'MENU' }]);
    expect(requestAuthorizedMenu(identity, 7)).toBe(first);
    clearAuthorizedMenuRequestCache(identity);
  });

  it('does not reuse a request when the permission version changes', async () => {
    const identity = { role: { root: false } };
    mockedGetMenuAuthorize
      .mockResolvedValueOnce([{ path: '/users', type: 'MENU' }])
      .mockResolvedValueOnce([{ path: '/role', type: 'MENU' }]);

    const previous = requestAuthorizedMenu(identity, 1);
    const refreshed = requestAuthorizedMenu(identity, 2);

    expect(refreshed).not.toBe(previous);
    await expect(previous).resolves.toEqual([{ path: '/users', type: 'MENU' }]);
    await expect(refreshed).resolves.toEqual([{ path: '/role', type: 'MENU' }]);
    expect(mockedGetMenuAuthorize).toHaveBeenCalledTimes(2);
    clearAuthorizedMenuRequestCache(identity);
  });

  it('does not share cached menus across different identity objects', async () => {
    const firstIdentity = { role: { root: false } };
    const secondIdentity = { role: { root: false } };
    mockedGetMenuAuthorize
      .mockResolvedValueOnce([{ path: '/users', type: 'MENU' }])
      .mockResolvedValueOnce([{ path: '/role', type: 'MENU' }]);

    await requestAuthorizedMenu(firstIdentity, 3);
    await requestAuthorizedMenu(secondIdentity, 3);

    expect(mockedGetMenuAuthorize).toHaveBeenCalledTimes(2);
    clearAuthorizedMenuRequestCache(firstIdentity);
    clearAuthorizedMenuRequestCache(secondIdentity);
  });

  it('evicts a rejected request so the same identity and version can retry', async () => {
    const identity = { role: { root: false } };
    mockedGetMenuAuthorize
      .mockRejectedValueOnce(new Error('temporary failure'))
      .mockResolvedValueOnce([{ path: '/users', type: 'MENU' }]);

    await expect(requestAuthorizedMenu(identity, 5)).rejects.toThrow('temporary failure');
    await expect(requestAuthorizedMenu(identity, 5)).resolves.toEqual([
      { path: '/users', type: 'MENU' },
    ]);
    expect(mockedGetMenuAuthorize).toHaveBeenCalledTimes(2);
    clearAuthorizedMenuRequestCache(identity);
  });
});
