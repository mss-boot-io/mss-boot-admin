import {
  ADMIN_PERMISSIONS,
  canAccessPermissionMarkedRoute,
  hasEveryPermission,
  hasPermission,
} from './authorization';

describe('authorization', () => {
  it('fails closed for missing identities and permissions', () => {
    expect(hasPermission(undefined, '/role')).toBe(false);
    expect(hasPermission({}, '/role')).toBe(false);
    expect(hasPermission({ permissions: { '/role': false } }, '/role')).toBe(false);
    expect(hasPermission({ permissions: { '/role': true } }, undefined)).toBe(false);
  });

  it('requires a permission value to be exactly true', () => {
    expect(hasPermission({ permissions: { '/role': true } }, '/role')).toBe(true);
    expect(hasPermission({ permissions: { '/role': 'component' } }, '/role')).toBe(false);
    expect(hasPermission({ permissions: { '/role': 1 } }, '/role')).toBe(false);
  });

  it('allows root identities without materializing every permission', () => {
    expect(hasPermission({ role: { root: true } }, '/role/delete')).toBe(true);
    expect(
      hasEveryPermission({ role: { root: true } }, [
        ADMIN_PERMISSIONS.appConfigControl,
        ADMIN_PERMISSIONS.storageUpload,
      ]),
    ).toBe(true);
  });

  it('requires every permission for compound actions', () => {
    const identity = {
      permissions: {
        [ADMIN_PERMISSIONS.appConfigControl]: true,
        [ADMIN_PERMISSIONS.storageUpload]: false,
      },
    };
    expect(hasPermission(identity, ADMIN_PERMISSIONS.appConfigControl)).toBe(true);
    expect(
      hasEveryPermission(identity, [
        ADMIN_PERMISSIONS.appConfigControl,
        ADMIN_PERMISSIONS.storageUpload,
      ]),
    ).toBe(false);
  });

  it('evaluates permission-marked routes through the same helper', () => {
    const identity = { permissions: { '/departments/edit': true } };
    expect(canAccessPermissionMarkedRoute(identity, { permission: '/departments/edit' })).toBe(
      true,
    );
    expect(canAccessPermissionMarkedRoute(identity, { permission: '/departments/create' })).toBe(
      false,
    );
    expect(canAccessPermissionMarkedRoute(identity, {})).toBe(false);
    expect(
      canAccessPermissionMarkedRoute(identity, { permission: '/users/create', rootOnly: true }),
    ).toBe(false);
    expect(canAccessPermissionMarkedRoute({ role: { root: true } }, { rootOnly: true })).toBe(true);
  });
});
