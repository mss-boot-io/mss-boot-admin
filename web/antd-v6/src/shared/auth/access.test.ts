import { describe, expect, it } from 'vitest';
import { canAccessRoute, flattenAuthorizedMenu } from './access';

describe('route access', () => {
  it('keeps root-only and permission checks explicit', () => {
    expect(canAccessRoute({ id: 'root', root: true }, { rootOnly: true })).toBe(true);
    expect(canAccessRoute({ id: 'user', root: false }, { rootOnly: true })).toBe(false);
    expect(
      canAccessRoute({ id: 'user', permissions: ['user:read'] }, { permission: 'user:read' }),
    ).toBe(true);
  });

  it('flattens authorized menu nodes without resolving component strings', () => {
    const flattened = flattenAuthorizedMenu([{ path: '/parent', children: [{ path: '/child' }] }]);
    expect(flattened.map((item) => item.path)).toEqual(['/parent', '/child']);
  });
});
