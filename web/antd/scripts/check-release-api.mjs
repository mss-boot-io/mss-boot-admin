import { readFile, readdir } from 'node:fs/promises';
import { dirname, extname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const distRoot = join(projectRoot, 'dist');
const textExtensions = new Set(['.css', '.html', '.js', '.json', '.map']);
const forbiddenEndpoints = [
  {
    name: 'loopback API endpoint',
    pattern: /https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?/i,
  },
  {
    name: 'environment-specific MSS API endpoint',
    pattern: /https:\/\/admin-api(?:-alpha|-beta)?\.mss-boot-io\.top/i,
  },
];

const listFiles = async (directory) => {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(
    entries.map((entry) => {
      const target = join(directory, entry.name);
      return entry.isDirectory() ? listFiles(target) : [target];
    }),
  );
  return files.flat();
};

const failures = [];
for (const file of await listFiles(distRoot)) {
  if (!textExtensions.has(extname(file))) {
    continue;
  }
  const content = await readFile(file, 'utf8');
  for (const endpoint of forbiddenEndpoints) {
    if (endpoint.pattern.test(content)) {
      failures.push(`${relative(distRoot, file)} contains a ${endpoint.name}`);
    }
  }
}

if (failures.length > 0) {
  throw new Error(`Release API contract failed:\n${failures.join('\n')}`);
}

console.log('Release API contract: same-origin (no fixed local/alpha/beta API endpoint)');
