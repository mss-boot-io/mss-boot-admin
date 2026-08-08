import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, relative, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const scriptPath = fileURLToPath(import.meta.url);
const scriptDirectory = dirname(scriptPath);
const repositoryRoot = resolve(scriptDirectory, '..', '..');
const acceptancePath = resolve(scriptDirectory, 'pnpm-audit-acceptances.json');

export const expectedPnpmVersion = '9.15.9';

const blockingSeverities = new Set(['critical', 'high']);
const advisorySeverities = ['info', 'low', 'moderate', 'high', 'critical'];
const advisorySeveritySet = new Set(advisorySeverities);
const acceptanceScopes = new Set(['docs', 'web/antd']);
const acceptanceKeys = new Set([
  'advisory',
  'package',
  'severity',
  'patchedVersions',
  'scopes',
  'expiresOn',
  'reason',
]);
const auditReportKeys = new Set(['actions', 'advisories', 'muted', 'metadata']);
const auditMetadataKeys = new Set([
  'vulnerabilities',
  'dependencies',
  'devDependencies',
  'optionalDependencies',
  'totalDependencies',
]);
const githubAdvisoryPattern =
  /^GHSA-[23456789cfghjmpqrvwx]{4}-[23456789cfghjmpqrvwx]{4}-[23456789cfghjmpqrvwx]{4}$/;

const isPlainObject = (value) =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

const requirePlainObject = (value, label) => {
  if (!isPlainObject(value)) {
    throw new Error(`${label} must be an object.`);
  }
};

const requireExactKeys = (value, expectedKeys, label) => {
  const unknown = Object.keys(value).filter((key) => !expectedKeys.has(key));
  if (unknown.length > 0) {
    throw new Error(`${label} contains unknown fields: ${unknown.join(', ')}.`);
  }
  const missing = [...expectedKeys].filter((key) => !Object.hasOwn(value, key));
  if (missing.length > 0) {
    throw new Error(`${label} is missing fields: ${missing.join(', ')}.`);
  }
};

const requireNonEmptyString = (value, label) => {
  if (typeof value !== 'string' || value.trim().length === 0) {
    throw new Error(`${label} must be a non-empty string.`);
  }
};

const requireNonNegativeInteger = (value, label) => {
  if (!Number.isInteger(value) || value < 0) {
    throw new Error(`${label} must be a non-negative integer.`);
  }
};

const isRealISODate = (value) => {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) {
    return false;
  }
  const [, yearText, monthText, dayText] = match;
  const year = Number(yearText);
  const month = Number(monthText);
  const day = Number(dayText);
  const date = new Date(0);
  date.setUTCHours(0, 0, 0, 0);
  date.setUTCFullYear(year, month - 1, day);
  return (
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day
  );
};

export const validateAcceptanceDocument = (document) => {
  requirePlainObject(document, 'pnpm audit acceptance document');
  requireExactKeys(
    document,
    new Set(['schemaVersion', 'acceptances']),
    'pnpm audit acceptance document',
  );
  if (document.schemaVersion !== 1) {
    throw new Error('Unsupported pnpm audit acceptance schema version.');
  }
  if (!Array.isArray(document.acceptances)) {
    throw new Error('pnpm audit acceptances must be an array.');
  }

  const seenAdvisories = new Set();
  for (const [index, acceptance] of document.acceptances.entries()) {
    const label = `pnpm audit acceptance at index ${index}`;
    requirePlainObject(acceptance, label);
    requireExactKeys(acceptance, acceptanceKeys, label);
    if (
      typeof acceptance.advisory !== 'string' ||
      !githubAdvisoryPattern.test(acceptance.advisory)
    ) {
      throw new Error(`${label}.advisory must be a valid GitHub advisory identifier.`);
    }
    if (seenAdvisories.has(acceptance.advisory)) {
      throw new Error(`Duplicate pnpm audit acceptance advisory: ${acceptance.advisory}.`);
    }
    seenAdvisories.add(acceptance.advisory);

    requireNonEmptyString(acceptance.package, `${label}.package`);
    if (!blockingSeverities.has(acceptance.severity)) {
      throw new Error(`${label}.severity must be high or critical.`);
    }
    requireNonEmptyString(acceptance.patchedVersions, `${label}.patchedVersions`);
    requireNonEmptyString(acceptance.reason, `${label}.reason`);
    if (!isRealISODate(acceptance.expiresOn)) {
      throw new Error(`${label}.expiresOn must be a real date in YYYY-MM-DD format.`);
    }
    if (!Array.isArray(acceptance.scopes) || acceptance.scopes.length === 0) {
      throw new Error(`${label}.scopes must be a non-empty array.`);
    }
    const seenScopes = new Set();
    for (const scope of acceptance.scopes) {
      if (typeof scope !== 'string' || !acceptanceScopes.has(scope)) {
        throw new Error(`${label}.scopes contains unsupported scope: ${String(scope)}.`);
      }
      if (seenScopes.has(scope)) {
        throw new Error(`${label}.scopes contains duplicate scope: ${scope}.`);
      }
      seenScopes.add(scope);
    }
  }

  return document;
};

export const validateAuditReport = (report) => {
  requirePlainObject(report, 'pnpm audit report');
  if (Object.hasOwn(report, 'error')) {
    const detail = isPlainObject(report.error)
      ? report.error.summary ?? report.error.message ?? JSON.stringify(report.error)
      : String(report.error);
    throw new Error(`pnpm audit reported an error: ${detail}`);
  }
  requireExactKeys(report, auditReportKeys, 'pnpm audit report');
  if (!Array.isArray(report.actions) || !Array.isArray(report.muted)) {
    throw new Error('pnpm audit report actions and muted fields must be arrays.');
  }
  requirePlainObject(report.advisories, 'pnpm audit report advisories');
  requirePlainObject(report.metadata, 'pnpm audit report metadata');
  requireExactKeys(report.metadata, auditMetadataKeys, 'pnpm audit report metadata');
  requirePlainObject(report.metadata.vulnerabilities, 'pnpm audit vulnerability metadata');
  requireExactKeys(
    report.metadata.vulnerabilities,
    new Set(advisorySeverities),
    'pnpm audit vulnerability metadata',
  );

  const counts = Object.fromEntries(advisorySeverities.map((severity) => [severity, 0]));
  const seenAdvisories = new Set();
  const advisories = Object.values(report.advisories);
  for (const [index, advisory] of advisories.entries()) {
    const label = `pnpm audit advisory at index ${index}`;
    requirePlainObject(advisory, label);
    if (!githubAdvisoryPattern.test(advisory.github_advisory_id)) {
      throw new Error(`${label}.github_advisory_id must be a valid GitHub advisory identifier.`);
    }
    requireNonEmptyString(advisory.module_name, `${label}.module_name`);
    requireNonEmptyString(advisory.patched_versions, `${label}.patched_versions`);
    if (!advisorySeveritySet.has(advisory.severity)) {
      throw new Error(`${label}.severity is unsupported: ${String(advisory.severity)}.`);
    }
    if (seenAdvisories.has(advisory.github_advisory_id)) {
      throw new Error(`pnpm audit report contains duplicate advisory: ${advisory.github_advisory_id}.`);
    }
    seenAdvisories.add(advisory.github_advisory_id);
    counts[advisory.severity] += 1;
  }

  for (const severity of advisorySeverities) {
    const reported = report.metadata.vulnerabilities[severity];
    requireNonNegativeInteger(reported, `pnpm audit metadata.vulnerabilities.${severity}`);
    if (reported !== counts[severity]) {
      throw new Error(
        `pnpm audit ${severity} count mismatch: metadata=${reported}, advisories=${counts[severity]}.`,
      );
    }
  }
  for (const field of [
    'dependencies',
    'devDependencies',
    'optionalDependencies',
    'totalDependencies',
  ]) {
    requireNonNegativeInteger(report.metadata[field], `pnpm audit metadata.${field}`);
  }
  const dependencyTotal =
    report.metadata.dependencies +
    report.metadata.devDependencies +
    report.metadata.optionalDependencies;
  if (report.metadata.totalDependencies !== dependencyTotal) {
    throw new Error(
      `pnpm audit dependency count mismatch: total=${report.metadata.totalDependencies}, components=${dependencyTotal}.`,
    );
  }

  return advisories;
};

export const assertSpawnCompleted = (result, label, allowedStatuses = new Set([0])) => {
  if (result.error) {
    throw new Error(`${label} failed to start: ${result.error.message}`);
  }
  if (result.signal !== null) {
    throw new Error(`${label} was terminated by signal ${result.signal}.`);
  }
  if (!Number.isInteger(result.status)) {
    throw new Error(`${label} did not return an exit status.`);
  }
  if (!allowedStatuses.has(result.status)) {
    const stderr = typeof result.stderr === 'string' ? result.stderr.trim() : '';
    throw new Error(`${label} exited with status ${result.status}${stderr ? `: ${stderr}` : '.'}`);
  }
};

export const validatePnpmVersion = (packageManager, reportedVersion) => {
  if (packageManager !== `pnpm@${expectedPnpmVersion}`) {
    throw new Error(`package.json must pin packageManager to pnpm@${expectedPnpmVersion}.`);
  }
  if (typeof reportedVersion !== 'string' || reportedVersion.trim() !== expectedPnpmVersion) {
    throw new Error(
      `Release dependency audit requires pnpm ${expectedPnpmVersion}; subprocess reported ${
        typeof reportedVersion === 'string' && reportedVersion.trim()
          ? reportedVersion.trim()
          : '<empty>'
      }.`,
    );
  }
};

const spawnPnpm = (pnpmScript, arguments_, packageDirectory) =>
  spawnSync(process.execPath, [pnpmScript, ...arguments_], {
    cwd: packageDirectory,
    encoding: 'utf8',
    maxBuffer: 32 * 1024 * 1024,
  });

const validatePnpmRuntime = (pnpmScript, packageDirectory) => {
  const packageDocument = JSON.parse(
    readFileSync(resolve(packageDirectory, 'package.json'), 'utf8'),
  );
  const result = spawnPnpm(pnpmScript, ['--version'], packageDirectory);
  assertSpawnCompleted(result, 'pnpm version check');
  validatePnpmVersion(packageDocument.packageManager, result.stdout);
};

const runAudit = (pnpmScript, packageDirectory, productionOnly) => {
  const arguments_ = ['audit', '--json', '--audit-level=critical'];
  if (productionOnly) {
    arguments_.push('--prod');
  }
  const result = spawnPnpm(pnpmScript, arguments_, packageDirectory);
  if (result.error) {
    assertSpawnCompleted(result, 'pnpm audit');
  }
  if (result.signal !== null) {
    assertSpawnCompleted(result, 'pnpm audit');
  }
  if (!result.stdout?.trim()) {
    assertSpawnCompleted(result, 'pnpm audit');
    throw new Error(`pnpm audit did not return JSON: ${result.stderr?.trim() ?? ''}`);
  }

  let report;
  try {
    report = JSON.parse(result.stdout);
  } catch (error) {
    throw new Error(`Unable to parse pnpm audit JSON: ${error.message}`);
  }
  const advisories = validateAuditReport(report);
  const allowedStatuses = advisories.length > 0 ? new Set([0, 1]) : new Set([0]);
  assertSpawnCompleted(result, 'pnpm audit', allowedStatuses);
  return advisories;
};

const parseAcceptanceDocument = () => {
  let document;
  try {
    document = JSON.parse(readFileSync(acceptancePath, 'utf8'));
  } catch (error) {
    throw new Error(`Unable to parse pnpm audit acceptances: ${error.message}`);
  }
  return validateAcceptanceDocument(document);
};

export const main = () => {
  const packageDirectory = process.cwd();
  const packageScope = relative(repositoryRoot, packageDirectory).replaceAll('\\', '/');
  if (!acceptanceScopes.has(packageScope)) {
    throw new Error(`Release dependency audit does not support package scope: ${packageScope}.`);
  }
  const pnpmScript = process.env.npm_execpath;
  if (!pnpmScript) {
    throw new Error('Run this check through the package script so npm_execpath identifies pnpm.');
  }
  validatePnpmRuntime(pnpmScript, packageDirectory);

  const productionAdvisories = runAudit(pnpmScript, packageDirectory, true);
  const productionBlockers = productionAdvisories.filter((advisory) =>
    blockingSeverities.has(advisory.severity),
  );
  if (productionBlockers.length > 0) {
    const identifiers = productionBlockers.map((item) => item.github_advisory_id).join(', ');
    throw new Error(`Production dependency audit has high or critical advisories: ${identifiers}`);
  }

  const acceptanceDocument = parseAcceptanceDocument();
  const today = new Date().toISOString().slice(0, 10);
  const acceptedForScope = new Map(
    acceptanceDocument.acceptances
      .filter((item) => item.scopes.includes(packageScope))
      .map((item) => [item.advisory, item]),
  );

  const fullAdvisories = runAudit(pnpmScript, packageDirectory, false);
  const fullBlockers = fullAdvisories.filter((advisory) =>
    blockingSeverities.has(advisory.severity),
  );
  const unexpected = [];
  for (const advisory of fullBlockers) {
    const identifier = advisory.github_advisory_id;
    const acceptance = acceptedForScope.get(identifier);
    if (
      !acceptance ||
      acceptance.expiresOn < today ||
      acceptance.package !== advisory.module_name ||
      acceptance.severity !== advisory.severity ||
      acceptance.patchedVersions !== advisory.patched_versions
    ) {
      unexpected.push(identifier);
    }
  }

  if (unexpected.length > 0) {
    throw new Error(`Unaccepted high or critical build advisories: ${unexpected.join(', ')}`);
  }

  const observed = new Set(fullBlockers.map((item) => item.github_advisory_id));
  const staleAcceptances = [...acceptedForScope.keys()].filter(
    (identifier) => !observed.has(identifier),
  );
  if (staleAcceptances.length > 0) {
    throw new Error(`Remove stale pnpm audit acceptances: ${staleAcceptances.join(', ')}`);
  }

  console.log(
    `Release dependency audit passed for ${packageScope}: 0 production blockers, ${fullBlockers.length} accepted build-only advisories.`,
  );
};

if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  main();
}
