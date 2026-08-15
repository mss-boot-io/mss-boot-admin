import { readdir, readFile } from 'node:fs/promises';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const manifest = JSON.parse(await readFile(join(projectRoot, 'package.json'), 'utf8'));
const lockfile = await readFile(join(projectRoot, 'pnpm-lock.yaml'), 'utf8');

const expected = {
  '@ant-design/icons': '6.3.2',
  '@ant-design/pro-components': '3.1.14-6',
  '@tanstack/react-query': '5.101.4',
  antd: '6.6.0',
  'antd-style': '4.1.0',
  react: '19.2.8',
  'react-dom': '19.2.8',
  '@umijs/max': '4.7.5',
  typescript: '7.0.2',
  vitest: '4.1.10',
};

const failures = [];
const declared = { ...manifest.dependencies, ...manifest.devDependencies };
const exactVersion = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;

const listTypeScript = async (directory) => {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(
    entries
      .filter((entry) => entry.name !== '.umi' && entry.name !== '.umi-production')
      .map((entry) => {
        const target = join(directory, entry.name);
        return entry.isDirectory()
          ? listTypeScript(target)
          : /\.tsx?$/.test(entry.name)
            ? [target]
            : [];
      }),
  );
  return files.flat();
};

for (const source of await listTypeScript(join(projectRoot, 'src'))) {
  const contents = await readFile(source, 'utf8');
  if (/(?:from\s*|import\s*\(\s*)['"]@ant-design\/icons['"]/.test(contents)) {
    failures.push(
      `${relative(projectRoot, source)} imports the icon barrel; use a public icon subpath`,
    );
  }
}

for (const [name, version] of Object.entries(declared)) {
  if (!exactVersion.test(version)) {
    failures.push(`${name} must use an exact version, found ${version}`);
  }
}

for (const [name, version] of Object.entries(expected)) {
  if (declared[name] !== version) {
    failures.push(`${name} must be ${version}, found ${declared[name] ?? 'missing'}`);
  }
}

if (manifest.packageManager !== 'pnpm@10.34.5') {
  failures.push(`packageManager must be pnpm@10.34.5, found ${manifest.packageManager}`);
}
if (!String(manifest.engines?.node).includes('24')) {
  failures.push('the Node engine must remain pinned to the Node 24 release line');
}
if (Number(process.versions.node.split('.')[0]) !== 24) {
  failures.push(`dependency checks require Node 24, found ${process.versions.node}`);
}
if (
  process.env.npm_config_user_agent &&
  !process.env.npm_config_user_agent.startsWith('pnpm/10.34.5 ')
) {
  failures.push(
    `dependency checks require pnpm 10.34.5, found ${process.env.npm_config_user_agent}`,
  );
}

const packageSection = lockfile.split('\nsnapshots:\n', 1)[0];
const versionsFor = (name) => {
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const matcher = new RegExp(`^  ['"]?${escaped}@([^:'"(]+)`, 'gm');
  return [...packageSection.matchAll(matcher)].map((match) => match[1]).sort();
};

const allowedResolvedVersions = {
  react: ['18.3.1', '19.2.8'],
  'react-dom': ['18.3.1', '19.2.8'],
  antd: ['4.24.16', '6.6.0'],
  '@ant-design/pro-components': ['3.1.14-6'],
  '@tanstack/react-query': ['5.101.4'],
  immer: ['9.0.21'],
};

for (const [name, allowed] of Object.entries(allowedResolvedVersions)) {
  const resolved = versionsFor(name);
  const unexpected = resolved.filter((version) => !allowed.includes(version));
  if (unexpected.length > 0) {
    failures.push(`${name} resolves unexpected versions: ${unexpected.join(', ')}`);
  }
  if (!resolved.includes(expected[name] ?? allowed.at(-1))) {
    failures.push(
      `${name} does not resolve the application version ${expected[name] ?? allowed.at(-1)}`,
    );
  }
}

for (const [name, version] of Object.entries(expected)) {
  try {
    const installed = JSON.parse(
      await readFile(join(projectRoot, 'node_modules', name, 'package.json'), 'utf8'),
    ).version;
    if (installed !== version) {
      failures.push(`${name} installed ${installed}, expected ${version}`);
    }
  } catch (error) {
    failures.push(`${name} is not installed: ${error.message}`);
  }
}

if (failures.length > 0) {
  throw new Error(`Dependency contract failed:\n${failures.join('\n')}`);
}

console.log(
  'Dependency contract passed: exact application versions and reviewed Umi transition graph',
);
console.log(
  'Known build-tool-only transition: antd 4.24.16 and React 18.3.1 remain under Umi packages',
);
