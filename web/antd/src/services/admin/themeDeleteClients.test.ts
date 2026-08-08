const mockRequest = jest.fn();

jest.mock('@umijs/max', () => ({
  request: (...args: unknown[]) => mockRequest(...args),
}));

import { deleteAppConfigsTheme, putAppConfigsGroup } from './appConfig';
import { deleteUserConfigsTheme, putUserConfigsGroup } from './userConfig';

describe('theme reset clients', () => {
  beforeEach(() => {
    mockRequest.mockReset();
    mockRequest.mockResolvedValue(undefined);
  });

  it('uses static application and user theme endpoints', async () => {
    await deleteAppConfigsTheme();
    await deleteUserConfigsTheme();

    expect(mockRequest).toHaveBeenNthCalledWith(1, '/admin/api/app-configs/theme', {
      method: 'DELETE',
    });
    expect(mockRequest).toHaveBeenNthCalledWith(2, '/admin/api/user-configs/theme', {
      method: 'DELETE',
    });
  });

  it('merges concurrency headers without dropping the JSON content type', async () => {
    const options = {
      headers: { 'If-Match': '"theme-application-8"' },
      skipErrorHandler: true,
    };
    await putAppConfigsGroup({ group: 'theme' }, { data: { fixedHeader: true } }, options);
    await putUserConfigsGroup(
      { group: 'theme' },
      { data: { fixedHeader: true } },
      { ...options, headers: { 'If-Match': '"theme-user-4"' } },
    );

    expect(mockRequest).toHaveBeenNthCalledWith(
      1,
      '/admin/api/app-configs/theme',
      expect.objectContaining({
        headers: {
          'Content-Type': 'application/json',
          'If-Match': '"theme-application-8"',
        },
        skipErrorHandler: true,
      }),
    );
    expect(mockRequest).toHaveBeenNthCalledWith(
      2,
      '/admin/api/user-configs/theme',
      expect.objectContaining({
        headers: {
          'Content-Type': 'application/json',
          'If-Match': '"theme-user-4"',
        },
        skipErrorHandler: true,
      }),
    );
  });
});
