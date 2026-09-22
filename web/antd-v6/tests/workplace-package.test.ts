import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const require = createRequire(import.meta.url);
const { defineBusinessAdmin } = require('../package/business.cjs');

describe('workplace package bridge', () => {
  it('defaults to no contributions in the reference application', () => {
    const config = defineBusinessAdmin();
    expect(config.alias['@mss-admin-business/workplace']).toBe(
      resolve(import.meta.dirname, '../package/empty-workplace.ts'),
    );
    expect(config.mssAdmin.workplaceContributions).toBe(
      config.alias['@mss-admin-business/workplace'],
    );
  });
  it('accepts an explicit trusted compile-time module without replacing core routes', () => {
    const config = defineBusinessAdmin({ workplaceContributions: './src/example-workplace.ts' });
    expect(config.alias['@mss-admin-business/workplace']).toBe(resolve('src/example-workplace.ts'));
    expect(() => defineBusinessAdmin({ workplaceContributions: 42 })).toThrow();
    expect(() => defineBusinessAdmin({ routes: [] })).toThrow();
  });
});
