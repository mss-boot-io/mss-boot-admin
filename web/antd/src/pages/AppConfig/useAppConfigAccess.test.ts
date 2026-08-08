import { renderHook } from '@testing-library/react';
import { useModel } from '@umijs/max';
import {
  APP_CONFIG_SECRET_FIELDS,
  omitAppConfigSecrets,
  prepareAppConfigSecretPayload,
  useAppConfigAccess,
} from './useAppConfigAccess';

jest.mock('@umijs/max', () => ({ useModel: jest.fn() }));

describe('useAppConfigAccess', () => {
  it('separates configuration writes from the compound logo upload permission', () => {
    (useModel as jest.Mock).mockReturnValue({
      initialState: {
        currentUser: {
          permissions: {
            '/app-config/control': true,
            '/storage/upload': false,
          },
        },
      },
    });

    expect(renderHook(() => useAppConfigAccess()).result.current).toEqual({
      canWrite: true,
      canUpload: false,
      canReadSecrets: false,
      canWriteSecrets: false,
    });
  });

  it('allows both operations for root and rejects non-boolean permission values', () => {
    (useModel as jest.Mock).mockReturnValueOnce({
      initialState: { currentUser: { role: { root: true } } },
    });
    expect(renderHook(() => useAppConfigAccess()).result.current).toEqual({
      canWrite: true,
      canUpload: true,
      canReadSecrets: true,
      canWriteSecrets: true,
    });

    (useModel as jest.Mock).mockReturnValueOnce({
      initialState: {
        currentUser: {
          permissions: { '/app-config/control': 'component', '/storage/upload': true },
        },
      },
    });
    expect(renderHook(() => useAppConfigAccess()).result.current).toEqual({
      canWrite: false,
      canUpload: false,
      canReadSecrets: false,
      canWriteSecrets: false,
    });
  });

  it('keeps secret read and write permissions independent while requiring control to write', () => {
    (useModel as jest.Mock).mockReturnValueOnce({
      initialState: {
        currentUser: {
          permissions: {
            '/app-config/control': true,
            '/app-config/secrets/read': true,
            '/app-config/secrets/write': false,
          },
        },
      },
    });
    expect(renderHook(() => useAppConfigAccess()).result.current).toMatchObject({
      canReadSecrets: true,
      canWriteSecrets: false,
    });

    (useModel as jest.Mock).mockReturnValueOnce({
      initialState: {
        currentUser: {
          permissions: {
            '/app-config/control': true,
            '/app-config/secrets/write': true,
          },
        },
      },
    });
    expect(renderHook(() => useAppConfigAccess()).result.current).toMatchObject({
      canReadSecrets: false,
      canWriteSecrets: true,
    });

    (useModel as jest.Mock).mockReturnValueOnce({
      initialState: {
        currentUser: { permissions: { '/app-config/secrets/write': true } },
      },
    });
    expect(renderHook(() => useAppConfigAccess()).result.current.canWriteSecrets).toBe(false);
  });

  it('removes every declared secret without mutating the source config', () => {
    const security = {
      githubClientId: 'client',
      githubClientSecret: 'github-secret',
      larkAppSecret: 'lark-secret',
    };
    expect(omitAppConfigSecrets('security', security)).toEqual({ githubClientId: 'client' });
    expect(security).toHaveProperty('githubClientSecret', 'github-secret');
    expect(omitAppConfigSecrets('email', { username: 'admin', password: 'secret' })).toEqual({
      username: 'admin',
    });
    expect(
      omitAppConfigSecrets('storage', { s3Bucket: 'bucket', s3SecretAccessKey: 'secret' }),
    ).toEqual({ s3Bucket: 'bucket' });
  });
});

describe('prepareAppConfigSecretPayload', () => {
  const groups = [
    ['email', APP_CONFIG_SECRET_FIELDS.email],
    ['security', APP_CONFIG_SECRET_FIELDS.security],
    ['storage', APP_CONFIG_SECRET_FIELDS.storage],
  ] as const;

  it.each(groups)('protects every %s secret field across the access matrix', (group, fields) => {
    const secrets = Object.fromEntries(
      fields.map((field, index) => [field, `rotated-secret-${index}`]),
    );
    const values = { ordinary: 'updated', ...secrets };

    expect(
      prepareAppConfigSecretPayload(group, values, {
        canReadSecrets: false,
        canWriteSecrets: false,
      }),
    ).toEqual({ ordinary: 'updated' });
    expect(
      prepareAppConfigSecretPayload(group, values, {
        canReadSecrets: true,
        canWriteSecrets: false,
      }),
    ).toEqual({ ordinary: 'updated' });
    expect(
      prepareAppConfigSecretPayload(group, values, {
        canReadSecrets: false,
        canWriteSecrets: true,
      }),
    ).toEqual(values);

    for (const emptyValue of ['', '   ', undefined, null]) {
      const emptySecrets = Object.fromEntries(fields.map((field) => [field, emptyValue]));
      expect(
        prepareAppConfigSecretPayload(
          group,
          { ordinary: 'updated', ...emptySecrets },
          { canReadSecrets: false, canWriteSecrets: true },
        ),
      ).toEqual({ ordinary: 'updated' });
    }

    const explicitClear = Object.fromEntries(
      fields.map((field, index) => [field, index % 2 === 0 ? '' : null]),
    );
    expect(
      prepareAppConfigSecretPayload(
        group,
        { ordinary: 'updated', ...explicitClear },
        { canReadSecrets: true, canWriteSecrets: true },
      ),
    ).toEqual({ ordinary: 'updated', ...explicitClear });
  });

  it('keeps only the explicitly entered security secret during a partial blind rotation', () => {
    expect(
      prepareAppConfigSecretPayload(
        'security',
        {
          githubClientId: 'client',
          githubClientSecret: 'rotated-github-secret',
          larkAppSecret: '',
        },
        { canReadSecrets: false, canWriteSecrets: true },
      ),
    ).toEqual({
      githubClientId: 'client',
      githubClientSecret: 'rotated-github-secret',
    });
  });
});
