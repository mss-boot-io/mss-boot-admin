import { readdir, readFile } from 'node:fs/promises';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const distRoot = join(projectRoot, 'dist');

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
const stats = JSON.parse(await readFile(join(distRoot, 'stats.json'), 'utf8'));
const moduleNames = Array.isArray(stats.modules)
  ? stats.modules.map((module) => String(module.name ?? ''))
  : [];

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
if (!moduleNames.some((name) => name.includes('/node_modules/dayjs/'))) {
  failures.push('the aliased Day.js runtime is missing from the production graph');
}

if (failures.length > 0) throw new Error(`Runtime bundle contract failed:\n${failures.join('\n')}`);
console.log(
  'Runtime bundle contract passed: React 19, Ant Design 6, and Day.js without Moment or legacy core-js',
);
