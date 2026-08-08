import { deleteAppConfigsTheme, getAppConfigsGroup, putAppConfigsGroup } from './appConfig';
import { deleteUserConfigsTheme, getUserConfigsGroup, putUserConfigsGroup } from './userConfig';
import { createThemeSettingsAdapter } from './themeSettings';

jest.mock('./appConfig', () => ({
  deleteAppConfigsTheme: jest.fn(),
  getAppConfigsGroup: jest.fn(),
  putAppConfigsGroup: jest.fn(),
}));

jest.mock('./userConfig', () => ({
  deleteUserConfigsTheme: jest.fn(),
  getUserConfigsGroup: jest.fn(),
  putUserConfigsGroup: jest.fn(),
}));

describe('theme settings adapters', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('uses application group endpoints and sparse null reset', async () => {
    (getAppConfigsGroup as jest.Mock).mockResolvedValue({
      fixedHeader: 'false',
      _meta: { v: 1, scope: 'application', revision: '1' },
    });
    (putAppConfigsGroup as jest.Mock)
      .mockResolvedValueOnce({
        fixedHeader: false,
        navTheme: 'light',
        _meta: { v: 1, scope: 'application', revision: '2' },
      })
      .mockResolvedValueOnce({
        navTheme: 'light',
        _meta: { v: 1, scope: 'application', revision: '3' },
      });
    (deleteAppConfigsTheme as jest.Mock).mockResolvedValue({
      _meta: { v: 1, scope: 'application', revision: '4' },
    });
    const adapter = createThemeSettingsAdapter('application');

    const base = await adapter.load();
    await expect(adapter.patch({ navTheme: 'light' }, base)).resolves.toMatchObject({
      revision: '2',
      overrides: { fixedHeader: false, navTheme: 'light' },
    });
    await adapter.reset(['fixedHeader'], { ...base, revision: '2' });
    await adapter.reset(undefined, { ...base, revision: '3' });

    expect(getAppConfigsGroup).toHaveBeenCalledWith(
      { group: 'theme' },
      {
        headers: { Accept: 'application/vnd.mss.theme.v1+json' },
        skipErrorHandler: true,
      },
    );
    expect(putAppConfigsGroup).toHaveBeenNthCalledWith(
      1,
      { group: 'theme' },
      { data: { navTheme: 'light' } },
      {
        headers: {
          'If-Match': '"theme-application-1"',
          Accept: 'application/vnd.mss.theme.v1+json',
        },
        skipErrorHandler: true,
      },
    );
    expect(putAppConfigsGroup).toHaveBeenNthCalledWith(
      2,
      { group: 'theme' },
      { data: { fixedHeader: null } },
      {
        headers: {
          'If-Match': '"theme-application-2"',
          Accept: 'application/vnd.mss.theme.v1+json',
        },
        skipErrorHandler: true,
      },
    );
    expect(deleteAppConfigsTheme).toHaveBeenCalledWith({
      headers: {
        'If-Match': '"theme-application-3"',
        Accept: 'application/vnd.mss.theme.v1+json',
      },
      skipErrorHandler: true,
    });
    expect(getUserConfigsGroup).not.toHaveBeenCalled();
  });

  it('uses current-user group endpoints', async () => {
    (getUserConfigsGroup as jest.Mock).mockResolvedValue({
      navTheme: 'realDark',
      _meta: { v: 1, scope: 'user', revision: '7' },
    });
    (putUserConfigsGroup as jest.Mock).mockResolvedValue({
      navTheme: 'realDark',
      colorWeak: false,
      _meta: { v: 1, scope: 'user', revision: '8' },
    });
    (deleteUserConfigsTheme as jest.Mock).mockResolvedValue({
      _meta: { v: 1, scope: 'user', revision: '9' },
    });
    const adapter = createThemeSettingsAdapter('user');

    const base = await adapter.load();
    expect(base).toMatchObject({ revision: '7', overrides: { navTheme: 'realDark' } });
    await adapter.patch({ colorWeak: false }, base);
    await adapter.reset(undefined, { ...base, revision: '8' });

    expect(getUserConfigsGroup).toHaveBeenCalledWith(
      { group: 'theme' },
      {
        headers: { Accept: 'application/vnd.mss.theme.v1+json' },
        skipErrorHandler: true,
      },
    );
    expect(putUserConfigsGroup).toHaveBeenCalledWith(
      { group: 'theme' },
      { data: { colorWeak: false } },
      {
        headers: {
          'If-Match': '"theme-user-7"',
          Accept: 'application/vnd.mss.theme.v1+json',
        },
        skipErrorHandler: true,
      },
    );
    expect(deleteUserConfigsTheme).toHaveBeenCalledWith({
      headers: {
        'If-Match': '"theme-user-8"',
        Accept: 'application/vnd.mss.theme.v1+json',
      },
      skipErrorHandler: true,
    });
    expect(getAppConfigsGroup).not.toHaveBeenCalled();
  });

  it('recognizes a BizError revision conflict and exposes its canonical current resource', async () => {
    const client = {
      get: jest.fn(),
      put: jest.fn().mockRejectedValue({
        name: 'BizError',
        info: {
          errorCode: 'THEME_REVISION_CONFLICT',
          data: {
            current: {
              fixedHeader: true,
              _meta: { v: 1, scope: 'application', revision: '10' },
            },
          },
        },
      }),
      delete: jest.fn(),
    };
    const adapter = createThemeSettingsAdapter('application', client);
    const base = {
      scope: 'application' as const,
      revision: '9',
      overrides: { fixedHeader: false },
      versioned: true,
    };

    await expect(adapter.patch({ fixedHeader: true }, base)).rejects.toMatchObject({
      name: 'ThemeRevisionConflictError',
      current: expect.objectContaining({
        revision: '10',
        overrides: { fixedHeader: true },
      }),
    });
  });
});
