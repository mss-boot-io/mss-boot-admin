import { configUmiAlias, createConfig } from '@umijs/max/test';

export default async () => {
  const config = await configUmiAlias({
    ...createConfig({
      target: 'browser',
    }),
  });

  console.log();
  return {
    ...config,
    // WSL may inherit TEMP/TMP from Windows. Keeping Jest's cache inside the
    // Linux workspace avoids very slow 9P filesystem access during transforms.
    cacheDirectory: '<rootDir>/node_modules/.cache/jest',
    testEnvironmentOptions: {
      ...(config?.testEnvironmentOptions || {}),
      url: 'http://localhost:8000',
    },
    setupFiles: [...(config.setupFiles || []), './tests/setupTests.jsx'],
  };
};
