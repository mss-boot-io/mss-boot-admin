import { readdir, readFile } from 'node:fs/promises';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const argumentValue = (name) => {
  const index = process.argv.indexOf(name);
  if (index < 0) return undefined;
  const value = process.argv[index + 1];
  if (!value || value.startsWith('--')) throw new Error(`${name} requires a path`);
  return value;
};
const distRoot = resolve(argumentValue('--dist') || join(projectRoot, 'dist'));
const statsPath = resolve(argumentValue('--stats') || join(distRoot, 'stats.json'));
const manifest = JSON.parse(await readFile(join(projectRoot, 'package.json'), 'utf8'));
const buildOnlyDependencies = manifest.mssAdminDistribution?.buildOnlyDependencies;
if (
  !buildOnlyDependencies ||
  typeof buildOnlyDependencies !== 'object' ||
  Array.isArray(buildOnlyDependencies) ||
  Object.keys(buildOnlyDependencies).length === 0
) {
  throw new Error('Admin Web package must declare mssAdminDistribution.buildOnlyDependencies');
}

const listJavaScript = async (directory) => {
  const entries = await readdir(directory, { withFileTypes: true });
  return (
    await Promise.all(
      entries.map((entry) => {
        const target = join(directory, entry.name);
        return entry.isDirectory()
          ? listJavaScript(target)
          : target.endsWith('.js')
            ? [target]
            : [];
      }),
    )
  ).flat();
};

const markers = [
  { name: 'React 19.2.8', value: '19.2.8', required: true },
  { name: 'Ant Design 6.6.0', value: '6.6.0', required: true },
  { name: 'transitional React 18 runtime', value: '18.3.1', required: false },
  { name: 'transitional Ant Design 4 runtime', value: '4.24.16', required: false },
];
const files = await listJavaScript(distRoot);
const contents = await Promise.all(
  files.map(async (file) => ({ file, content: await readFile(file, 'utf8') })),
);
const failures = [];
const stats = JSON.parse(await readFile(statsPath, 'utf8'));
const collectModuleNames = (modules) =>
  Array.isArray(modules)
    ? modules.flatMap((module) => [
        String(module.name ?? ''),
        ...collectModuleNames(module.modules),
      ])
    : [];
const moduleNames = collectModuleNames(stats.modules);

for (const marker of markers) {
  const matches = contents.filter(({ content }) => content.includes(marker.value));
  if (marker.required && matches.length === 0) {
    failures.push(`${marker.name} is missing from the production bundle`);
  }
  if (!marker.required && matches.length > 0) {
    failures.push(
      `${marker.name} entered: ${matches.map(({ file }) => relative(distRoot, file)).join(', ')}`,
    );
  }
}

const momentRuntime = moduleNames.filter((name) => name.includes('/node_modules/moment/'));
if (momentRuntime.length > 0) {
  failures.push(`Moment entered the production graph: ${momentRuntime.slice(0, 3).join(', ')}`);
}
const coreJsRuntime = moduleNames.filter((name) => name.includes('/node_modules/core-js/'));
if (coreJsRuntime.length > 0) {
  failures.push(
    `core-js entered the modern-browser production graph: ${coreJsRuntime.slice(0, 3).join(', ')}`,
  );
}
const legacyNormalizeRuntime = moduleNames.filter((name) =>
  name.includes('/es5-ext/string/#/normalize/'),
);
if (legacyNormalizeRuntime.length > 0) {
  failures.push(
    `the obsolete es5-ext Unicode normalization table entered the production graph: ${legacyNormalizeRuntime
      .slice(0, 3)
      .join(', ')}`,
  );
}
if (!moduleNames.some((name) => name.includes('/node_modules/dayjs/'))) {
  failures.push('the aliased Day.js runtime is missing from the production graph');
}
const axiosRuntime = moduleNames.filter((name) => name.includes('/node_modules/axios/'));
if (axiosRuntime.length === 0) {
  failures.push('the governed Axios request runtime is missing from the production graph');
} else {
  const unexpectedAxiosRuntime = axiosRuntime.filter(
    (name) => !name.includes('.pnpm/axios@0.33.0/node_modules/axios/'),
  );
  if (unexpectedAxiosRuntime.length > 0) {
    failures.push(
      `an ungoverned Axios version entered the production graph: ${unexpectedAxiosRuntime
        .slice(0, 3)
        .join(', ')}`,
    );
  }
}
const escapeRegExp = (value) => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
for (const [packageName, versions] of Object.entries(buildOnlyDependencies)) {
  if (!Array.isArray(versions) || versions.length === 0) {
    failures.push(`build-only dependency ${packageName} must declare exact versions`);
    continue;
  }
  const storeName = packageName.replace('/', '+');
  const versionMatchers = versions.map(
    (version) =>
      new RegExp(
        `/node_modules/\\.pnpm/${escapeRegExp(storeName)}@${escapeRegExp(version)}(?:_[^/]*)?/node_modules/${escapeRegExp(packageName)}/`,
      ),
  );
  const buildToolRuntime = moduleNames.filter((name) =>
    versionMatchers.some((matcher) => matcher.test(name)),
  );
  if (buildToolRuntime.length > 0) {
    failures.push(
      `${packageName} accepted build-only resolution entered the runtime graph: ${buildToolRuntime
        .slice(0, 3)
        .join(', ')}`,
    );
  }
}
for (const packageName of ['mockjs']) {
  const buildToolRuntime = moduleNames.filter((name) =>
    name.includes(`/node_modules/${packageName}/`),
  );
  if (buildToolRuntime.length > 0) {
    failures.push(
      `${packageName} must remain build-only: ${buildToolRuntime.slice(0, 3).join(', ')}`,
    );
  }
}

if (failures.length > 0) throw new Error(`Runtime bundle contract failed:\n${failures.join('\n')}`);
console.log(
  'Runtime bundle contract passed: governed React, Ant Design, Day.js, and Axios runtimes without retired or build-only packages',
);
