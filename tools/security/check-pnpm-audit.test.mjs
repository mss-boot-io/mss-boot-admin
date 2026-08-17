import assert from 'node:assert/strict';
import test from 'node:test';

import {
  assertSpawnCompleted,
  validateAcceptanceDocument,
  validateAuditReport,
  validatePnpmVersion,
} from './check-pnpm-audit.mjs';

const validAcceptance = () => ({
  schemaVersion: 1,
  acceptances: [
    {
      advisory: 'GHSA-w3rx-r6r6-pgpr',
      package: 'image-size',
      severity: 'high',
      patchedVersions: '<0.0.0',
      scopes: ['docs', 'web/antd-v6'],
      expiresOn: '2026-11-08',
      reason: 'No patched release exists and this is build-only.',
    },
  ],
});

const validReport = () => ({
  actions: [],
  advisories: {
    1: {
      github_advisory_id: 'GHSA-w3rx-r6r6-pgpr',
      module_name: 'image-size',
      severity: 'high',
      patched_versions: '<0.0.0',
    },
  },
  muted: [],
  metadata: {
    vulnerabilities: { info: 0, low: 0, moderate: 0, high: 1, critical: 0 },
    dependencies: 10,
    devDependencies: 20,
    optionalDependencies: 0,
    totalDependencies: 30,
  },
});

test('accepts valid acceptance and audit documents', () => {
  assert.equal(validateAcceptanceDocument(validAcceptance()).acceptances.length, 1);
  assert.equal(validateAuditReport(validReport()).length, 1);
});

test('requires each package pin and subprocess to use its governed pnpm version', () => {
  assert.doesNotThrow(() => validatePnpmVersion('pnpm@9.15.9', '9.15.9\n', '9.15.9'));
  assert.doesNotThrow(() => validatePnpmVersion('pnpm@10.34.5', '10.34.5\n', '10.34.5'));
  assert.throws(() => validatePnpmVersion('pnpm@9.15.8', '9.15.9', '9.15.9'), /must pin/);
  assert.throws(
    () => validatePnpmVersion('pnpm@9.15.9', '10.0.0', '9.15.9'),
    /subprocess reported 10.0.0/,
  );
  assert.throws(
    () => validatePnpmVersion('pnpm@10.34.5', '', '10.34.5'),
    /subprocess reported <empty>/,
  );
});

test('rejects unknown or missing acceptance fields', () => {
  const document = validAcceptance();
  document.extra = true;
  assert.throws(() => validateAcceptanceDocument(document), /unknown fields: extra/);

  const missing = validAcceptance();
  delete missing.acceptances[0].reason;
  assert.throws(() => validateAcceptanceDocument(missing), /missing fields: reason/);
});

test('rejects invalid dates, scopes, empty reasons, and duplicate advisories', () => {
  const invalidDate = validAcceptance();
  invalidDate.acceptances[0].expiresOn = '2026-02-30';
  assert.throws(() => validateAcceptanceDocument(invalidDate), /real date/);

  const duplicateScope = validAcceptance();
  duplicateScope.acceptances[0].scopes = ['docs', 'docs'];
  assert.throws(() => validateAcceptanceDocument(duplicateScope), /duplicate scope/);

  const unknownScope = validAcceptance();
  unknownScope.acceptances[0].scopes = ['unknown'];
  assert.throws(() => validateAcceptanceDocument(unknownScope), /unsupported scope/);

  const emptyReason = validAcceptance();
  emptyReason.acceptances[0].reason = '  ';
  assert.throws(() => validateAcceptanceDocument(emptyReason), /reason must be a non-empty string/);

  const duplicateAdvisory = validAcceptance();
  duplicateAdvisory.acceptances.push({ ...duplicateAdvisory.acceptances[0] });
  assert.throws(() => validateAcceptanceDocument(duplicateAdvisory), /Duplicate.*advisory/);
});

test('rejects audit error reports and malformed advisory metadata', () => {
  assert.throws(
    () => validateAuditReport({ error: { summary: 'registry unavailable' } }),
    /registry unavailable/,
  );

  const mismatch = validReport();
  mismatch.metadata.vulnerabilities.high = 0;
  assert.throws(() => validateAuditReport(mismatch), /high count mismatch/);

  const unknownReportField = validReport();
  unknownReportField.extra = true;
  assert.throws(() => validateAuditReport(unknownReportField), /unknown fields: extra/);

  const unknownMetadataField = validReport();
  unknownMetadataField.metadata.extra = 0;
  assert.throws(() => validateAuditReport(unknownMetadataField), /unknown fields: extra/);

  const totalMismatch = validReport();
  totalMismatch.metadata.totalDependencies = 31;
  assert.throws(() => validateAuditReport(totalMismatch), /dependency count mismatch/);

  const invalidIdentifier = validReport();
  invalidIdentifier.advisories[1].github_advisory_id = 'not-a-ghsa';
  assert.throws(() => validateAuditReport(invalidIdentifier), /valid GitHub advisory identifier/);

  const unknownSeverity = validReport();
  unknownSeverity.advisories[1].severity = 'unknown';
  assert.throws(() => validateAuditReport(unknownSeverity), /severity is unsupported/);

  const duplicate = validReport();
  duplicate.advisories[2] = { ...duplicate.advisories[1] };
  duplicate.metadata.vulnerabilities.high = 2;
  assert.throws(() => validateAuditReport(duplicate), /duplicate advisory/);
});

test('rejects every abnormal spawn termination', () => {
  assert.throws(
    () => assertSpawnCompleted({ error: new Error('ENOENT'), signal: null, status: null }, 'pnpm'),
    /failed to start: ENOENT/,
  );
  assert.throws(
    () => assertSpawnCompleted({ error: undefined, signal: 'SIGTERM', status: null }, 'pnpm'),
    /terminated by signal SIGTERM/,
  );
  assert.throws(
    () => assertSpawnCompleted({ error: undefined, signal: null, status: null }, 'pnpm'),
    /did not return an exit status/,
  );
  assert.throws(
    () => assertSpawnCompleted({ error: undefined, signal: null, status: 2, stderr: 'bad' }, 'pnpm'),
    /exited with status 2: bad/,
  );
  assert.doesNotThrow(() =>
    assertSpawnCompleted({ error: undefined, signal: null, status: 0, stderr: '' }, 'pnpm'),
  );
  assert.doesNotThrow(() =>
    assertSpawnCompleted(
      { error: undefined, signal: null, status: 1, stderr: '' },
      'pnpm audit',
      new Set([0, 1]),
    ),
  );
});
