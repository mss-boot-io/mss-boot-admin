import { loadUserDependencies, resolveUserDependencyAccess } from './dependencies';

describe('user dependency loading', () => {
  it('does not request unrelated options for a desktop list-only identity', async () => {
    const access = resolveUserDependencyAccess(
      { permissions: { '/users': true } },
      { isForm: false, isMobile: false },
    );
    const loaders = {
      roles: jest.fn().mockResolvedValue({ data: [] }),
      posts: jest.fn().mockResolvedValue({ data: [{ id: 'post-1' }] }),
      departments: jest.fn().mockResolvedValue({ data: [] }),
    };

    const result = await loadUserDependencies(access, loaders, false);

    expect(loaders.roles).not.toHaveBeenCalled();
    expect(loaders.posts).not.toHaveBeenCalled();
    expect(loaders.departments).not.toHaveBeenCalled();
    expect(result).toEqual({ values: {}, errors: {} });
  });

  it('requests only the list enrichments authorized for the identity', async () => {
    const access = resolveUserDependencyAccess(
      { permissions: { '/users': true, '/posts': true } },
      { isForm: false, isMobile: false },
    );
    const loaders = {
      roles: jest.fn().mockResolvedValue({ data: [] }),
      posts: jest.fn().mockResolvedValue({ data: [{ id: 'post-1' }] }),
      departments: jest.fn().mockResolvedValue({ data: [] }),
    };

    const result = await loadUserDependencies(access, loaders, false);

    expect(loaders.roles).not.toHaveBeenCalled();
    expect(loaders.posts).toHaveBeenCalledTimes(1);
    expect(loaders.departments).not.toHaveBeenCalled();
    expect(result.values.posts).toEqual({ data: [{ id: 'post-1' }] });
  });

  it('keeps list enrichment usable when one permitted dependency fails', async () => {
    const access = { roles: true, posts: true, departments: true };
    const rolesError = new Error('roles forbidden');

    await expect(
      loadUserDependencies(
        access,
        {
          roles: jest.fn().mockRejectedValue(rolesError),
          posts: jest.fn().mockResolvedValue({ data: [{ id: 'post-1' }] }),
          departments: jest.fn().mockResolvedValue({ data: [{ id: 'dept-1' }] }),
        },
        false,
      ),
    ).resolves.toEqual({
      values: {
        posts: { data: [{ id: 'post-1' }] },
        departments: { data: [{ id: 'dept-1' }] },
      },
      errors: { roles: rolesError },
    });
  });

  it('blocks a form on partial failure and succeeds after retry', async () => {
    const roles = jest
      .fn()
      .mockRejectedValueOnce(new Error('temporary failure'))
      .mockResolvedValueOnce({ data: [{ id: 'role-1' }] });
    const loaders = {
      roles,
      posts: jest.fn().mockResolvedValue({ data: [] }),
      departments: jest.fn().mockResolvedValue({ data: [] }),
    };
    const access = resolveUserDependencyAccess(
      { permissions: { '/users': true } },
      { isForm: true, isMobile: false },
    );

    await expect(loadUserDependencies(access, loaders, true)).rejects.toThrow(
      'temporary failure',
    );
    await expect(loadUserDependencies(access, loaders, true)).resolves.toMatchObject({
      values: { roles: { data: [{ id: 'role-1' }] } },
      errors: {},
    });
    expect(roles).toHaveBeenCalledTimes(2);
  });
});
