import { describe, expect, it } from 'vitest';
import { IdentityContractError, normalizeCurrentUser } from './identity';

describe('current user contract', () => {
  it('uses the backend role and permission-map shapes', () => {
    const user = normalizeCurrentUser({
      id: 'user-1',
      roleID: 'role-1',
      role: { id: 'role-1', name: 'operator', root: true },
      permissions: { '/welcome': true, '/users': false, ignored: 'true' },
    });

    expect(user.role?.root).toBe(true);
    expect(user.permissions).toEqual({ '/welcome': true, '/users': false });
  });

  it('does not accept legacy client-side privilege shapes', () => {
    const user = normalizeCurrentUser({
      id: 'user-1',
      root: true,
      role: { root: false },
      permissions: ['/welcome'],
    });

    expect(user.role?.root).toBe(false);
    expect(user.permissions).toEqual({});
  });

  it('rejects malformed identities instead of creating an anonymous-looking user', () => {
    expect(() => normalizeCurrentUser({ username: 'missing-id' })).toThrow(IdentityContractError);
    expect(() => normalizeCurrentUser(null)).toThrow(IdentityContractError);
  });
});
