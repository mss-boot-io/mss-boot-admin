import {
  createRoleAuthorizationAdapter,
  formatRoleAuthorizationETag,
  normalizeRoleAuthorizationPaths,
  parseRoleAuthorizationResource,
} from './roleAuthorization';

describe('role authorization adapter', () => {
  it('normalizes the canonical resource and formats its strong ETag', () => {
    const resource = parseRoleAuthorizationResource(
      {
        roleID: 'role-1',
        paths: [' /users ', '/role', '/users', null],
        revision: '7',
      },
      'role-1',
    );

    expect(resource).toEqual({
      roleID: 'role-1',
      paths: ['/role', '/users'],
      revision: '7',
      versioned: true,
    });
    expect(formatRoleAuthorizationETag(resource)).toBe('"role-authorization-role-1-7"');
  });

  it('sends If-Match and keeps an explicit empty path set', async () => {
    const client = {
      get: jest.fn(),
      post: jest.fn().mockResolvedValue({ roleID: 'role-1', paths: [], revision: '8' }),
    };
    const adapter = createRoleAuthorizationAdapter(client);

    await expect(
      adapter.save([], {
        roleID: 'role-1',
        paths: ['/users'],
        revision: '7',
        versioned: true,
      }),
    ).resolves.toMatchObject({ paths: [], revision: '8' });

    expect(client.post).toHaveBeenCalledWith('role-1', [], {
      headers: {
        'Content-Type': 'application/json',
        'If-Match': '"role-authorization-role-1-7"',
      },
      skipErrorHandler: true,
    });
  });

  it('reloads after a legacy mutation response', async () => {
    const client = {
      get: jest.fn().mockResolvedValue({ roleID: 'role-1', paths: ['/role'] }),
      post: jest.fn().mockResolvedValue(undefined),
    };
    const adapter = createRoleAuthorizationAdapter(client);

    await expect(
      adapter.save(['/role'], {
        roleID: 'role-1',
        paths: [],
        revision: '',
        versioned: false,
      }),
    ).resolves.toMatchObject({ paths: ['/role'], versioned: false });
    expect(client.get).toHaveBeenCalledWith('role-1', { skipErrorHandler: true });
  });

  it('exposes the latest resource from a 412 without replacing the caller draft', async () => {
    const conflict = {
      response: { status: 412 },
      info: {
        errorCode: 'AUTHORIZATION_REVISION_CONFLICT',
        data: {
          current: { roleID: 'role-1', paths: ['/menu'], revision: '9' },
        },
      },
    };
    const adapter = createRoleAuthorizationAdapter({
      get: jest.fn(),
      post: jest.fn().mockRejectedValue(conflict),
    });

    await expect(
      adapter.save(['/users'], {
        roleID: 'role-1',
        paths: ['/role'],
        revision: '7',
        versioned: true,
      }),
    ).rejects.toMatchObject({
      name: 'RoleAuthorizationRevisionConflictError',
      current: { paths: ['/menu'], revision: '9' },
    });
  });

  it('parses the canonical HTTP 412 response body', async () => {
    const conflict = {
      response: {
        status: 412,
        data: {
          success: false,
          status: 'error',
          code: 412,
          errorCode: 'AUTHORIZATION_REVISION_CONFLICT',
          errorMessage: 'role authorization changed since it was loaded',
          traceId: 'trace-1',
          data: {
            current: { roleID: 'role-1', paths: ['/menu'], revision: '10' },
          },
        },
      },
    };
    const adapter = createRoleAuthorizationAdapter({
      get: jest.fn(),
      post: jest.fn().mockRejectedValue(conflict),
    });

    await expect(
      adapter.save(['/users'], {
        roleID: 'role-1',
        paths: ['/role'],
        revision: '9',
        versioned: true,
      }),
    ).rejects.toMatchObject({
      name: 'RoleAuthorizationRevisionConflictError',
      current: { roleID: 'role-1', paths: ['/menu'], revision: '10' },
    });
  });

  it('rejects a canonical resource for another role', () => {
    expect(() =>
      parseRoleAuthorizationResource({ roleID: 'role-2', paths: [], revision: '1' }, 'role-1'),
    ).toThrow('Invalid role authorization resource identity');
    expect(() =>
      parseRoleAuthorizationResource({ roleID: 'role-1', revision: 'not-a-number' }, 'role-1'),
    ).toThrow('Invalid role authorization resource revision');
    expect(() =>
      parseRoleAuthorizationResource({ roleID: 'role-1', revision: '1' }, 'role-1'),
    ).toThrow('Invalid canonical role authorization resource');
    expect(normalizeRoleAuthorizationPaths(null)).toEqual([]);
  });
});
