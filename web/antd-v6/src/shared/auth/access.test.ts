import { describe, expect, it } from 'vitest';
import { canAccessRoute, flattenAuthorizedMenu } from './access';

describe('route access', () => {
  it('keeps root-only and permission checks explicit', () => {
    expect(
      canAccessRoute({ id: 'root', role: { root: true }, permissions: {} }, { rootOnly: true }),
    ).toBe(true);
    expect(
      canAccessRoute({ id: 'user', role: { root: false }, permissions: {} }, { rootOnly: true }),
    ).toBe(false);
    expect(
      canAccessRoute({ id: 'user', permissions: { '/users': true } }, { permission: '/users' }),
    ).toBe(true);
    expect(
      canAccessRoute({ id: 'user', permissions: { '/users': false } }, { permission: '/users' }),
    ).toBe(false);
  });

  it('flattens authorized menu nodes without resolving component strings', () => {
    const flattened = flattenAuthorizedMenu([{ path: '/parent', children: [{ path: '/child' }] }]);
    expect(flattened.map((item) => item.path)).toEqual(['/parent', '/child']);
  });
});
