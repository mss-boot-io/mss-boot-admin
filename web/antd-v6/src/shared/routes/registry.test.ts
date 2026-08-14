import { describe, expect, it } from 'vitest';
import { retainRegisteredMenu } from './registry';

describe('compiled route registry', () => {
  it('drops unknown executable menu paths while retaining registered children', () => {
    const retained = retainRegisteredMenu([
      {
        path: '/unknown-parent',
        children: [{ path: '/workplace' }, { path: '/database-component' }],
      },
    ]);
    expect(retained).toEqual([
      { path: '/unknown-parent', children: [{ path: '/workplace', children: undefined }] },
    ]);
  });
});
