import { readFile, readdir } from 'node:fs/promises';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const projectRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const distRoot = join(projectRoot, 'dist');
// Keep this aligned with gzip_comp_level in nginx.conf so the budget reflects
// bytes transferred by the production container rather than an idealized archive.
const productionGzipLevel = 6;

const readBudget = (name, fallbackKiB) => {
  const raw = process.env[name];
  const value = raw === undefined ? fallbackKiB : Number(raw);
  if (!Number.isFinite(value) || value <= 0) {
    throw new Error(`${name} must be a positive number of KiB`);
  }
  return value * 1024;
};

const entryBudget = readBudget('MSS_ENTRY_GZIP_BUDGET_KB', 575);
const chunkBudget = readBudget('MSS_CHUNK_GZIP_BUDGET_KB', 250);

const formatKiB = (bytes) => `${(bytes / 1024).toFixed(2)} KiB`;

const listJavaScript = async (directory) => {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(
    entries.map((entry) => {
      const target = join(directory, entry.name);
      return entry.isDirectory() ? listJavaScript(target) : target.endsWith('.js') ? [target] : [];
    }),
  );
  return files.flat();
};

const html = await readFile(join(distRoot, 'index.html'), 'utf8');
const entryMatch = html.match(/<script[^>]+src=["']([^"']*\/umi\.[^"']+\.js)["']/i);

if (!entryMatch) {
  throw new Error('Unable to locate the hashed Umi entry script in dist/index.html');
}

const entryPath = join(distRoot, entryMatch[1].replace(/^\//, ''));
const files = await listJavaScript(distRoot);
const sizes = await Promise.all(
  files.map(async (file) => ({
    file,
    gzip: gzipSync(await readFile(file), { level: productionGzipLevel }).byteLength,
  })),
);
const entry = sizes.find(({ file }) => file === entryPath);

if (!entry) {
  throw new Error(`Entry script is missing from the build output: ${entryPath}`);
}

const largestChunk = sizes
  .filter(({ file }) => file !== entryPath)
  .sort((left, right) => right.gzip - left.gzip)[0];
const failures = [];

if (entry.gzip > entryBudget) {
  failures.push(`entry ${formatKiB(entry.gzip)} exceeds ${formatKiB(entryBudget)}`);
}
if (largestChunk && largestChunk.gzip > chunkBudget) {
  failures.push(
    `${relative(distRoot, largestChunk.file)} ${formatKiB(largestChunk.gzip)} exceeds ${formatKiB(chunkBudget)}`,
  );
}

console.log(`Bundle entry: ${formatKiB(entry.gzip)}`);
if (largestChunk) {
  console.log(
    `Largest async chunk: ${relative(distRoot, largestChunk.file)} (${formatKiB(largestChunk.gzip)})`,
  );
}

if (failures.length > 0) {
  console.error(`Bundle budget failed: ${failures.join('; ')}`);
  process.exitCode = 1;
}
