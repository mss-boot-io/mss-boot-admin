import { describe, expect, it } from 'vitest';
import { isPublicPath } from './session';

describe('isPublicPath', () => {
  it('keeps only explicit authentication routes public', () => {
    expect(isPublicPath('/user/login')).toBe(true);
    expect(isPublicPath('/user/callback/github')).toBe(true);
    expect(isPublicPath('/user/oauth/callback/github')).toBe(true);
    expect(isPublicPath('/user/callback/github/extra')).toBe(false);
    expect(isPublicPath('/workplace')).toBe(false);
    expect(isPublicPath('/users')).toBe(false);
  });
});
