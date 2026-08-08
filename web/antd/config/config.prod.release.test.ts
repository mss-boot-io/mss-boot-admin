jest.mock('@umijs/max', () => ({
  defineConfig: (value: unknown) => value,
}));

import config from './config.prod.release';

describe('release build configuration', () => {
  it('uses the deployment origin for API requests', () => {
    expect(config).toMatchObject({
      define: { API_URL: '' },
    });
  });
});
