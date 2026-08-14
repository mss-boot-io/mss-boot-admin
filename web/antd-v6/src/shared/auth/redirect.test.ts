import { describe, expect, it } from 'vitest';
import { resolveSafeRedirect } from './redirect';

describe('resolveSafeRedirect', () => {
  const origin = 'https://admin.example.test';

  it('preserves only same-origin paths', () => {
    expect(resolveSafeRedirect('/users?page=2#details', '/workplace', origin)).toBe(
      '/users?page=2#details',
    );
    expect(resolveSafeRedirect('https://evil.example/path', '/workplace', origin)).toBe(
      '/workplace',
    );
    expect(resolveSafeRedirect('//evil.example/path', '/workplace', origin)).toBe('/workplace');
    expect(resolveSafeRedirect('/\\evil.example/path', '/workplace', origin)).toBe('/workplace');
  });
});
