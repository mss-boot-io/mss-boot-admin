jest.mock('@umijs/max', () => ({
  defineConfig: (value: unknown) => value,
}));

import config from './config.prod.release';

const packageConfig = require('../package.json');

describe('release build configuration', () => {
  it('uses the deployment origin for API requests', () => {
    expect(config).toMatchObject({
      define: { API_URL: '' },
    });
  });

  it('generates Umi type surfaces before a standalone type-check', () => {
    expect(packageConfig.scripts.tsc).toBe('max setup && tsc --noEmit');
  });
});
