import { readdir, readFile } from 'node:fs/promises';
import { dirname, extname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const distRoot = join(projectRoot, 'dist');
const textExtensions = new Set(['.css', '.html', '.js', '.json', '.map']);
const forbiddenEndpoints = [
  {
    name: 'loopback API endpoint',
    pattern:
      /https?:\/\/(?:localhost|127\.0\.0\.1)(?:(?::\d+)(?:\/|["'])|\/(?:admin|api)(?:\/|["']))/i,
  },
  {
    name: 'environment-specific MSS API endpoint',
    pattern: /https:\/\/admin-api(?:-alpha|-beta)?\.mss-boot-io\.top/i,
  },
];

const listFiles = async (directory) => {
  const entries = await readdir(directory, { withFileTypes: true });
  return (
    await Promise.all(
      entries.map((entry) => {
        const target = join(directory, entry.name);
        return entry.isDirectory() ? listFiles(target) : [target];
      }),
    )
  ).flat();
};

const failures = [];
const files = await listFiles(distRoot);
for (const file of files) {
  if (!textExtensions.has(extname(file))) continue;
  const content = await readFile(file, 'utf8');
  for (const endpoint of forbiddenEndpoints) {
    const match = content.match(endpoint.pattern);
    if (match) {
      const start = Math.max(0, match.index - 80);
      const context = content.slice(start, match.index + match[0].length + 80).replace(/\s+/g, ' ');
      failures.push(`${relative(distRoot, file)} contains a ${endpoint.name}: ${context}`);
    }
  }
}

for (const forbiddenFile of ['service-worker.js', 'sw.js']) {
  if (files.some((file) => relative(distRoot, file) === forbiddenFile)) {
    failures.push(`${forbiddenFile} must not be emitted by the authenticated Admin application`);
  }
}

if (failures.length > 0) throw new Error(`Release API contract failed:\n${failures.join('\n')}`);
console.log('Release contract passed: same-origin API and no service worker');
