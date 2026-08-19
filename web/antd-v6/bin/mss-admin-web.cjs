#!/usr/bin/env node

const { spawnSync } = require('node:child_process');
const { existsSync, readFileSync } = require('node:fs');
const { dirname, resolve } = require('node:path');

const packageRoot = resolve(__dirname, '..');
const distributionManifest = require('../package.json');
const distributionContract = distributionManifest.mssAdminDistribution;

function runNode(modulePath, args, env = process.env) {
  const result = spawnSync(process.execPath, [require.resolve(modulePath), ...args], {
    cwd: process.cwd(),
    env,
    stdio: 'inherit',
  });
  if (result.error) throw result.error;
  process.exitCode = result.status ?? 1;
}

function validateHost() {
  const root = process.cwd();
  const hostManifestPath = resolve(root, 'package.json');
  const alternatives = [
    ['package manifest', ['package.json']],
    ['Umi configuration', ['config/config.ts', 'config/config.js', '.umirc.ts', '.umirc.js']],
    ['Admin runtime glue', ['src/app.tsx', 'src/app.ts']],
    ['Admin access glue', ['src/access.ts', 'src/access.tsx']],
    ['business route registry', ['src/generated/routes.ts']],
  ];
  const missing = alternatives
    .filter(
      ([, candidates]) => !candidates.some((candidate) => existsSync(resolve(root, candidate))),
    )
    .map(([label, candidates]) => `${label} (${candidates.join(' or ')})`);
  if (missing.length > 0) {
    throw new Error(`invalid Admin business host; missing ${missing.join(', ')}`);
  }
  const hostManifest = JSON.parse(readFileSync(hostManifestPath, 'utf8'));
  if (
    !distributionContract ||
    typeof distributionContract.packageManager !== 'string' ||
    !distributionContract.runtimeOverrides ||
    typeof distributionContract.runtimeOverrides !== 'object'
  ) {
    throw new Error('Admin distribution package is missing its published host contract');
  }
  if (hostManifest.packageManager !== distributionContract.packageManager) {
    throw new Error(
      `Admin business host must use ${distributionContract.packageManager}, found ${hostManifest.packageManager || 'no packageManager'}`,
    );
  }
  const requiredOverrides = distributionContract.runtimeOverrides;
  const hostOverrides = hostManifest.pnpm?.overrides || {};
  const mismatches = Object.entries(requiredOverrides)
    .filter(([name, version]) => hostOverrides[name] !== version)
    .map(([name, version]) => `${name}=${version}`);
  if (mismatches.length > 0) {
    throw new Error(
      `Admin business host must preserve the distribution's single-runtime pnpm overrides: ${mismatches.join(', ')}`,
    );
  }
}

function usage() {
  console.error('Usage: mss-admin-web <dev|lint|test|build> [arguments]');
  process.exitCode = 2;
}

const [command, ...args] = process.argv.slice(2);
switch (command) {
  case 'dev':
    validateHost();
    runNode('@umijs/max/dist/cli.js', ['dev', ...args], {
      ...process.env,
      PORT: process.env.PORT || '8001',
      REACT_APP_ENV: process.env.REACT_APP_ENV || 'dev',
      UMI_ENV: process.env.UMI_ENV || 'dev',
      MOCK: process.env.MOCK || 'none',
    });
    break;
  case 'build':
    validateHost();
    runNode('@umijs/max/dist/cli.js', ['build', ...args], {
      ...process.env,
      REACT_APP_ENV: process.env.REACT_APP_ENV || 'release',
      UMI_ENV: process.env.UMI_ENV || 'release',
    });
    if (process.exitCode) break;
    for (const script of [
      'check-runtime-bundle.mjs',
      'check-bundle-budget.mjs',
      'check-release-api.mjs',
    ]) {
      runNode(resolve(packageRoot, 'scripts', script), ['--dist', resolve(process.cwd(), 'dist')]);
      if (process.exitCode) break;
    }
    break;
  case 'test':
    validateHost();
    runNode(resolve(dirname(require.resolve('vitest')), 'vitest.mjs'), [
      'run',
      '--config',
      resolve(packageRoot, 'package/vitest.config.mjs'),
      ...args,
    ]);
    break;
  case 'lint': {
    validateHost();
    const biome = spawnSync(require.resolve('@biomejs/biome/bin/biome'), ['check', '.'], {
      cwd: process.cwd(),
      stdio: 'inherit',
    });
    if (biome.error) throw biome.error;
    if (biome.status !== 0) {
      process.exitCode = biome.status ?? 1;
      break;
    }
    runNode('@umijs/max/dist/cli.js', ['setup']);
    if (process.exitCode) break;
    runNode(resolve(dirname(require.resolve('typescript')), 'tsc.js'), ['--noEmit', ...args]);
    break;
  }
  default:
    usage();
}
