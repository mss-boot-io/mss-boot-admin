// Keep a unique basename: Umi uses it as the plugin identity.
const { copyFileSync, existsSync, readFileSync } = require('node:fs');
const { resolve } = require('node:path');

const packageRoot = resolve(__dirname, '..');
const coreSource = resolve(packageRoot, 'src');
const emptyRegistrations = resolve(__dirname, 'empty-route-registrations.ts');
const externalGlobal = resolve(__dirname, 'external-global.ts');
const logo = resolve(packageRoot, 'public/logo.svg');
const tailwindSource = resolve(coreSource, 'tailwind.css');

module.exports = function mssAdminPlugin(api) {
  api.describe({
    key: 'mssAdmin',
    config: {
      schema({ zod }) {
        return zod
          .object({
            packageRoot: zod.string(),
            routeRegistrations: zod.string(),
          })
          .partial();
      },
    },
    enableBy: api.EnableBy.config,
  });

  api.modifyConfig((memo) => {
    const registrations = resolve(api.cwd, memo.mssAdmin?.routeRegistrations || emptyRegistrations);
    const includes = new Set([...(memo.extraBabelIncludes || []), coreSource]);
    return {
      ...memo,
      alias: {
        ...(memo.alias || {}),
        '@mss-admin-core': coreSource,
        '@mss-admin-business/routes': registrations,
      },
      extraBabelIncludes: [...includes],
    };
  });

  api.modifyTSConfig((memo) => {
    const registrations = resolve(
      api.cwd,
      api.config.mssAdmin?.routeRegistrations || emptyRegistrations,
    );
    return {
      ...memo,
      compilerOptions: {
        ...(memo.compilerOptions || {}),
        paths: {
          ...(memo.compilerOptions?.paths || {}),
          '@mss-admin-core/*': [`${coreSource}/*`],
          '@mss-admin-business/routes': [registrations],
        },
      },
    };
  });

  if (resolve(api.paths.absSrcPath) !== coreSource) {
    const generatedTailwind = resolve(api.paths.absTmpPath, 'mss-admin-tailwind.css');
    api.onGenerateFiles(() => {
      const sourceDirective = '@source "./**/*.{ts,tsx}";';
      const sourcePath = (value) => value.replaceAll('\\', '/');
      const template = readFileSync(tailwindSource, 'utf8');
      const content = template.replace(
        sourceDirective,
        [
          `@source "${sourcePath(coreSource)}/**/*.{ts,tsx}";`,
          `@source "${sourcePath(api.paths.absSrcPath)}/**/*.{ts,tsx}";`,
        ].join('\n'),
      );
      if (content === template) {
        throw new Error('Admin Tailwind source directive is missing');
      }
      api.writeTmpFile({ noPluginDir: true, path: 'mss-admin-tailwind.css', content });
    });
    api.addEntryImportsAhead(() => [{ source: generatedTailwind }, { source: externalGlobal }]);
    api.addBeforeMiddlewares(() => [
      (request, response, next) => {
        if (request.path !== '/logo.svg') {
          next();
          return;
        }
        response.sendFile(logo);
      },
    ]);
    api.onBuildComplete(({ err }) => {
      if (!err) copyFileSync(logo, resolve(api.paths.absOutputPath, 'logo.svg'));
    });
  }

  api.addTmpGenerateWatcherPaths(() => {
    const registrations = resolve(
      api.cwd,
      api.config.mssAdmin?.routeRegistrations || emptyRegistrations,
    );
    return [coreSource, registrations, tailwindSource].filter((entry) => existsSync(entry));
  });
};
