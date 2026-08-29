import {
  ADMIN_PRESENTATION_API_VERSION,
  ADMIN_PRESENTATION_KIND,
  type AdminPagePresentationProfile,
  type PresentationLayer,
  type PresentationLayers,
} from './contract';

export type PresentationAdoptionMode = 'disabled' | 'shadow' | 'active';
export type PresentationAdoptionState =
  | 'recovery'
  | 'unknown-page'
  | 'disabled'
  | 'shadow'
  | 'not-allowlisted'
  | 'active';

export interface EffectivePresentationAdoption {
  mode: PresentationAdoptionMode;
  state: PresentationAdoptionState;
  resolveLayers: boolean;
  applyLayers: boolean;
}

export interface EffectivePresentationDiagnostic {
  code: string;
  layer?: PresentationLayer;
  expectedDefinitionHash?: string;
  observedDefinitionHash?: string;
  issues: readonly { code: string; path: string }[];
}

export interface EffectivePresentationResponse {
  pageKey: string;
  definitionHash?: string;
  adoption: EffectivePresentationAdoption;
  layers: PresentationLayers;
  diagnostics: readonly EffectivePresentationDiagnostic[];
}

export class EffectivePresentationContractError extends Error {
  constructor() {
    super('Invalid effective presentation response');
    this.name = 'EffectivePresentationContractError';
  }
}

const identifierPattern = /^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$/;
const hashPattern = /^sha256:[0-9a-f]{64}$/;
const diagnosticCodePattern = /^[a-z][a-z0-9-]{0,119}$/;

function invalid(): never {
  throw new EffectivePresentationContractError();
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function requiredString(value: Record<string, unknown>, key: string, maximum: number): string {
  const candidate = value[key];
  if (
    typeof candidate !== 'string' ||
    candidate.length === 0 ||
    candidate.trim() !== candidate ||
    [...candidate].length > maximum
  ) {
    return invalid();
  }
  return candidate;
}

function optionalHash(value: unknown): string | undefined {
  if (value === undefined || value === null || value === '') return undefined;
  if (typeof value !== 'string' || !hashPattern.test(value)) return invalid();
  return value;
}

function requiredBoolean(value: Record<string, unknown>, key: string): boolean {
  const candidate = value[key];
  if (typeof candidate !== 'boolean') return invalid();
  return candidate;
}

function sanitizeJSON(value: unknown, depth = 0): unknown {
  if (depth > 20) return invalid();
  if (value === null || typeof value === 'boolean' || typeof value === 'string') return value;
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) return invalid();
    return value;
  }
  if (Array.isArray(value)) {
    if (value.length > 2_000) return invalid();
    return value.map((entry) => sanitizeJSON(entry, depth + 1));
  }
  if (!isRecord(value) || Object.keys(value).length > 2_000) return invalid();
  const result: Record<string, unknown> = Object.create(null);
  for (const [key, nested] of Object.entries(value)) {
    if (!key || key === '__proto__' || key === 'constructor' || key === 'prototype')
      return invalid();
    result[key] = sanitizeJSON(nested, depth + 1);
  }
  return result;
}

function parseProfile(value: unknown): AdminPagePresentationProfile {
  const sanitized = sanitizeJSON(value);
  if (!isRecord(sanitized)) return invalid();
  if (
    sanitized.apiVersion !== ADMIN_PRESENTATION_API_VERSION ||
    sanitized.kind !== ADMIN_PRESENTATION_KIND ||
    !isRecord(sanitized.metadata) ||
    !isRecord(sanitized.spec)
  ) {
    return invalid();
  }
  requiredString(sanitized.metadata, 'name', 120);
  const pageKey = requiredString(sanitized.metadata, 'pageKey', 120);
  if (!identifierPattern.test(pageKey)) return invalid();
  if (!hashPattern.test(requiredString(sanitized.metadata, 'definitionHash', 71))) return invalid();
  if (!isRecord(sanitized.metadata.scope)) return invalid();
  const scope = requiredString(sanitized.metadata.scope, 'kind', 16);
  if (scope !== 'application' && scope !== 'role' && scope !== 'user') return invalid();
  if (scope !== 'application') requiredString(sanitized.metadata.scope, 'subject', 160);
  return sanitized as unknown as AdminPagePresentationProfile;
}

function parseLayers(value: unknown): PresentationLayers {
  if (!isRecord(value)) return invalid();
  const result: PresentationLayers = {};
  for (const layer of ['application', 'role', 'user'] as const) {
    const raw = value[layer];
    if (raw !== undefined && raw !== null) result[layer] = parseProfile(raw);
  }
  return result;
}

function parseDiagnostic(value: unknown): EffectivePresentationDiagnostic {
  if (!isRecord(value)) return invalid();
  const code = requiredString(value, 'code', 120);
  if (!diagnosticCodePattern.test(code)) return invalid();
  const layer = value.layer;
  if (
    layer !== undefined &&
    layer !== '' &&
    layer !== 'application' &&
    layer !== 'role' &&
    layer !== 'user'
  ) {
    return invalid();
  }
  const rawIssues = value.issues ?? [];
  if (!Array.isArray(rawIssues) || rawIssues.length > 100) return invalid();
  const issues = rawIssues.map((issue) => {
    if (!isRecord(issue)) return invalid();
    const issueCode = requiredString(issue, 'code', 120);
    const path = requiredString(issue, 'path', 512);
    if (!diagnosticCodePattern.test(issueCode)) return invalid();
    return { code: issueCode, path };
  });
  return {
    code,
    ...(layer ? { layer } : {}),
    ...(optionalHash(value.expectedDefinitionHash)
      ? { expectedDefinitionHash: optionalHash(value.expectedDefinitionHash) }
      : {}),
    ...(optionalHash(value.observedDefinitionHash)
      ? { observedDefinitionHash: optionalHash(value.observedDefinitionHash) }
      : {}),
    issues,
  } as EffectivePresentationDiagnostic;
}

function parseAdoption(value: unknown): EffectivePresentationAdoption {
  if (!isRecord(value)) return invalid();
  const mode = requiredString(value, 'mode', 16);
  const state = requiredString(value, 'state', 32);
  if (mode !== 'disabled' && mode !== 'shadow' && mode !== 'active') return invalid();
  if (
    state !== 'recovery' &&
    state !== 'unknown-page' &&
    state !== 'disabled' &&
    state !== 'shadow' &&
    state !== 'not-allowlisted' &&
    state !== 'active'
  ) {
    return invalid();
  }
  const resolveLayers = requiredBoolean(value, 'resolveLayers');
  const applyLayers = requiredBoolean(value, 'applyLayers');
  if (applyLayers && (!resolveLayers || mode !== 'active' || state !== 'active')) return invalid();
  if (state === 'active' && (!resolveLayers || !applyLayers || mode !== 'active')) return invalid();
  if (state === 'shadow' && (mode !== 'shadow' || !resolveLayers || applyLayers)) return invalid();
  if (state === 'disabled' && mode !== 'disabled') return invalid();
  if (state === 'not-allowlisted' && mode !== 'active') return invalid();
  if (
    (state === 'disabled' || state === 'not-allowlisted' || state === 'recovery') &&
    (resolveLayers || applyLayers)
  ) {
    return invalid();
  }
  return { mode, state, resolveLayers, applyLayers };
}

export function parseEffectivePresentationResponse(value: unknown): EffectivePresentationResponse {
  if (!isRecord(value)) return invalid();
  const pageKey = requiredString(value, 'pageKey', 120);
  if (!identifierPattern.test(pageKey)) return invalid();
  const diagnostics = value.diagnostics;
  if (!Array.isArray(diagnostics) || diagnostics.length > 100) return invalid();
  return {
    pageKey,
    definitionHash: optionalHash(value.definitionHash),
    adoption: parseAdoption(value.adoption),
    layers: parseLayers(value.layers),
    diagnostics: diagnostics.map(parseDiagnostic),
  };
}
