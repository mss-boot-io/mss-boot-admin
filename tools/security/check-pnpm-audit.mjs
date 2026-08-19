import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, relative, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const scriptPath = fileURLToPath(import.meta.url);
const scriptDirectory = dirname(scriptPath);
const repositoryRoot = resolve(scriptDirectory, '..', '..');
const acceptancePath = resolve(scriptDirectory, 'pnpm-audit-acceptances.json');

export const expectedPnpmVersions = new Map([
  ['docs', '9.15.9'],
  ['web/antd-v6', '10.34.5'],
]);

const blockingSeverities = new Set(['critical', 'high']);
const advisorySeverities = ['info', 'low', 'moderate', 'high', 'critical'];
const advisorySeveritySet = new Set(advisorySeverities);
const acceptanceScopes = new Set(expectedPnpmVersions.keys());
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
const dependencyClassKeys = new Set(['runtime', 'tooling']);
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

const requireSortedUniqueStrings = (value, label) => {
  if (!Array.isArray(value) || value.length === 0) {
    throw new Error(`${label} must be a non-empty array.`);
  }
  const normalized = [];
  const seen = new Set();
  for (const [index, item] of value.entries()) {
    requireNonEmptyString(item, `${label}[${index}]`);
    if (seen.has(item)) {
      throw new Error(`${label} contains duplicate package: ${item}.`);
    }
    seen.add(item);
    normalized.push(item);
  }
  const sorted = [...normalized].sort();
  if (normalized.some((item, index) => item !== sorted[index])) {
    throw new Error(`${label} must use stable sorted order.`);
  }
  return normalized;
};

const sameStrings = (left, right) =>
  left.length === right.length && left.every((item, index) => item === right[index]);

export const validateDistributionDependencyContract = (packageDocument) => {
  requirePlainObject(packageDocument, 'package.json');
  const distribution = packageDocument.mssAdminDistribution;
  requirePlainObject(distribution, 'package.json mssAdminDistribution');
  const dependencyClasses = distribution.dependencyClasses;
  requirePlainObject(dependencyClasses, 'mssAdminDistribution.dependencyClasses');
  requireExactKeys(
    dependencyClasses,
    dependencyClassKeys,
    'mssAdminDistribution.dependencyClasses',
  );
  const runtime = requireSortedUniqueStrings(
    dependencyClasses.runtime,
    'mssAdminDistribution.dependencyClasses.runtime',
  );
  const tooling = requireSortedUniqueStrings(
    dependencyClasses.tooling,
    'mssAdminDistribution.dependencyClasses.tooling',
  );
  const combined = [...runtime, ...tooling];
  const combinedSet = new Set(combined);
  if (combinedSet.size !== combined.length) {
    const overlap = runtime.filter((name) => tooling.includes(name));
    throw new Error(`Admin Web runtime and tooling dependency classes overlap: ${overlap.join(', ')}.`);
  }

  const productionRoots = Object.keys({
    ...(packageDocument.dependencies ?? {}),
    ...(packageDocument.optionalDependencies ?? {}),
  }).sort();
  const classifiedRoots = [...combined].sort();
  if (!sameStrings(productionRoots, classifiedRoots)) {
    const missing = productionRoots.filter((name) => !combinedSet.has(name));
    const unknown = classifiedRoots.filter((name) => !productionRoots.includes(name));
    throw new Error(
      `Admin Web dependency classes must exactly partition published dependencies; missing=${
        missing.join(', ') || '<none>'
      }; unknown=${unknown.join(', ') || '<none>'}.`,
    );
  }

  const buildOnlyDependencies = distribution.buildOnlyDependencies;
  requirePlainObject(buildOnlyDependencies, 'mssAdminDistribution.buildOnlyDependencies');
  const buildOnlyNames = Object.keys(buildOnlyDependencies);
  if (buildOnlyNames.length === 0) {
    throw new Error('mssAdminDistribution.buildOnlyDependencies must not be empty.');
  }
  const sortedBuildOnlyNames = [...buildOnlyNames].sort();
  if (buildOnlyNames.some((item, index) => item !== sortedBuildOnlyNames[index])) {
    throw new Error('mssAdminDistribution.buildOnlyDependencies must use stable sorted order.');
  }
  const buildOnlyResolutions = new Map(
    buildOnlyNames.map((name) => {
      requireNonEmptyString(name, 'mssAdminDistribution.buildOnlyDependencies package');
      return [
        name,
        new Set(
          requireSortedUniqueStrings(
            buildOnlyDependencies[name],
            `mssAdminDistribution.buildOnlyDependencies.${name}`,
          ),
        ),
      ];
    }),
  );
  const developmentRoots = Object.keys(packageDocument.devDependencies ?? {});
  return {
    runtimeRoots: new Set(runtime),
    toolingRoots: new Set(tooling),
    developmentRoots: new Set(developmentRoots),
    buildOnlyResolutions,
  };
};

const auditPathRoot = (findingPath) => {
  const parts = findingPath.split('>');
  if (parts.length < 2 || (parts[0] !== '.' && parts[0] !== 'root') || !parts[1]) {
    throw new Error(`Unsupported pnpm audit dependency path: ${findingPath}.`);
  }
  return parts[1];
};

export const classifyAuditAdvisory = (advisory, dependencyContract) => {
  let observedToolingPath = false;
  for (const finding of advisory.findings) {
    for (const findingPath of finding.paths) {
      const root = auditPathRoot(findingPath);
      if (dependencyContract.runtimeRoots.has(root)) {
        return 'runtime';
      }
      if (
        dependencyContract.toolingRoots.has(root) ||
        dependencyContract.developmentRoots.has(root)
      ) {
        observedToolingPath = true;
        continue;
      }
      throw new Error(
        `pnpm audit path ${findingPath} starts from unclassified dependency root ${root}.`,
      );
    }
  }
  if (!observedToolingPath) {
    throw new Error(`pnpm audit advisory ${advisory.github_advisory_id} has no classified paths.`);
  }
  return 'tooling';
};

export const validateBuildOnlyAcceptanceCoverage = (
  acceptedForScope,
  dependencyContract,
) => {
  const acceptedPackages = [...new Set([...acceptedForScope.values()].map((item) => item.package))]
    .sort();
  const declaredPackages = [...dependencyContract.buildOnlyResolutions.keys()].sort();
  if (!sameStrings(acceptedPackages, declaredPackages)) {
    const acceptedSet = new Set(acceptedPackages);
    const declaredSet = new Set(declaredPackages);
    const missing = acceptedPackages.filter((name) => !declaredSet.has(name));
    const stale = declaredPackages.filter((name) => !acceptedSet.has(name));
    throw new Error(
      `Admin Web build-only package contract must match audit acceptances; missing=${
        missing.join(', ') || '<none>'
      }; stale=${stale.join(', ') || '<none>'}.`,
    );
  }
};

export const validateBuildOnlyResolutionCoverage = (advisories, dependencyContract) => {
  const observed = new Map();
  for (const advisory of advisories) {
    const declaredVersions = dependencyContract.buildOnlyResolutions.get(advisory.module_name);
    if (!declaredVersions) {
      throw new Error(
        `Build-only advisory ${advisory.github_advisory_id} targets undeclared package ${advisory.module_name}.`,
      );
    }
    const observedVersions = observed.get(advisory.module_name) ?? new Set();
    for (const finding of advisory.findings) {
      requireNonEmptyString(
        finding.version,
        `pnpm audit advisory ${advisory.github_advisory_id} finding version`,
      );
      if (!declaredVersions.has(finding.version)) {
        throw new Error(
          `Build-only advisory ${advisory.github_advisory_id} observed undeclared ${advisory.module_name}@${finding.version}.`,
        );
      }
      observedVersions.add(finding.version);
    }
    observed.set(advisory.module_name, observedVersions);
  }

  for (const [name, declaredVersions] of dependencyContract.buildOnlyResolutions) {
    const expected = [...declaredVersions].sort();
    const actual = [...(observed.get(name) ?? [])].sort();
    if (!sameStrings(expected, actual)) {
      throw new Error(
        `Admin Web build-only resolution contract drifted for ${name}; expected=${
          expected.join(', ') || '<none>'
        }; observed=${actual.join(', ') || '<none>'}.`,
      );
    }
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

  const advisoryCounts = Object.fromEntries(advisorySeverities.map((severity) => [severity, 0]));
  const findingPathCounts = Object.fromEntries(
    advisorySeverities.map((severity) => [severity, 0]),
  );
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
    if (!Array.isArray(advisory.findings) || advisory.findings.length === 0) {
      throw new Error(`${label}.findings must be a non-empty array.`);
    }
    let findingPathCount = 0;
    for (const [findingIndex, finding] of advisory.findings.entries()) {
      const findingLabel = `${label}.findings[${findingIndex}]`;
      requirePlainObject(finding, findingLabel);
      if (!Array.isArray(finding.paths) || finding.paths.length === 0) {
        throw new Error(`${findingLabel}.paths must be a non-empty array.`);
      }
      for (const [pathIndex, findingPath] of finding.paths.entries()) {
        requireNonEmptyString(findingPath, `${findingLabel}.paths[${pathIndex}]`);
      }
      findingPathCount += finding.paths.length;
    }
    if (seenAdvisories.has(advisory.github_advisory_id)) {
      throw new Error(`pnpm audit report contains duplicate advisory: ${advisory.github_advisory_id}.`);
    }
    seenAdvisories.add(advisory.github_advisory_id);
    advisoryCounts[advisory.severity] += 1;
    findingPathCounts[advisory.severity] += findingPathCount;
  }

  for (const severity of advisorySeverities) {
    const reported = report.metadata.vulnerabilities[severity];
    requireNonNegativeInteger(reported, `pnpm audit metadata.vulnerabilities.${severity}`);
    if (reported !== advisoryCounts[severity] && reported !== findingPathCounts[severity]) {
      throw new Error(
        `pnpm audit ${severity} count mismatch: metadata=${reported}, uniqueAdvisories=${advisoryCounts[severity]}, findingPaths=${findingPathCounts[severity]}.`,
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

export const validatePnpmVersion = (packageManager, reportedVersion, expectedPnpmVersion) => {
  requireNonEmptyString(expectedPnpmVersion, 'expected pnpm version');
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

const validatePnpmRuntime = (pnpmScript, packageDirectory, expectedPnpmVersion) => {
  const packageDocument = JSON.parse(
    readFileSync(resolve(packageDirectory, 'package.json'), 'utf8'),
  );
  const result = spawnPnpm(pnpmScript, ['--version'], packageDirectory);
  assertSpawnCompleted(result, 'pnpm version check');
  validatePnpmVersion(packageDocument.packageManager, result.stdout, expectedPnpmVersion);
  return packageDocument;
};

export const buildAuditArguments = (productionOnly) => [
  'audit',
  '--json',
  ...(productionOnly ? ['--prod'] : []),
];

const runAudit = (pnpmScript, packageDirectory, productionOnly) => {
  const arguments_ = buildAuditArguments(productionOnly);
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
  const expectedPnpmVersion = expectedPnpmVersions.get(packageScope);
  const pnpmScript = process.env.npm_execpath;
  if (!pnpmScript) {
    throw new Error('Run this check through the package script so npm_execpath identifies pnpm.');
  }
  const packageDocument = validatePnpmRuntime(
    pnpmScript,
    packageDirectory,
    expectedPnpmVersion,
  );
  const dependencyContract =
    packageScope === 'web/antd-v6'
      ? validateDistributionDependencyContract(packageDocument)
      : undefined;

  const acceptanceDocument = parseAcceptanceDocument();
  const today = new Date().toISOString().slice(0, 10);
  const acceptedForScope = new Map(
    acceptanceDocument.acceptances
      .filter((item) => item.scopes.includes(packageScope))
      .map((item) => [item.advisory, item]),
  );
  if (dependencyContract) {
    validateBuildOnlyAcceptanceCoverage(acceptedForScope, dependencyContract);
  }

  const productionAdvisories = runAudit(pnpmScript, packageDirectory, true);
  const productionBlockers = productionAdvisories.filter((advisory) =>
    blockingSeverities.has(advisory.severity),
  );
  const runtimeBlockers = dependencyContract
    ? productionBlockers.filter(
        (advisory) => classifyAuditAdvisory(advisory, dependencyContract) === 'runtime',
      )
    : productionBlockers;
  if (runtimeBlockers.length > 0) {
    const identifiers = runtimeBlockers.map((item) => item.github_advisory_id).join(', ');
    throw new Error(`Production dependency audit has high or critical advisories: ${identifiers}`);
  }

  const fullAdvisories = runAudit(pnpmScript, packageDirectory, false);
  const fullBlockers = fullAdvisories.filter((advisory) =>
    blockingSeverities.has(advisory.severity),
  );
  const unexpected = [];
  const toolingBlockers = [];
  for (const advisory of fullBlockers) {
    const identifier = advisory.github_advisory_id;
    const acceptance = acceptedForScope.get(identifier);
    const classification = dependencyContract
      ? classifyAuditAdvisory(advisory, dependencyContract)
      : 'tooling';
    if (dependencyContract && classification === 'tooling') {
      toolingBlockers.push(advisory);
    }
    if (
      classification === 'runtime' ||
      (dependencyContract &&
        !dependencyContract.buildOnlyResolutions.has(advisory.module_name)) ||
      !acceptance ||
      acceptance.expiresOn < today ||
      acceptance.package !== advisory.module_name ||
      acceptance.severity !== advisory.severity ||
      acceptance.patchedVersions !== advisory.patched_versions
    ) {
      unexpected.push(identifier);
    }
  }

  if (dependencyContract) {
    validateBuildOnlyResolutionCoverage(toolingBlockers, dependencyContract);
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

  const boundary = dependencyContract ? 'runtime' : 'production';
  console.log(
    `Release dependency audit passed for ${packageScope}: 0 ${boundary} blockers, ${fullBlockers.length} accepted build-only advisories.`,
  );
};

if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  main();
}
