import {
  type PageCapabilityDefinition,
  validatePageCapabilityDefinition,
} from '@mss-admin-core/shared/presentation/contract';

export const PRESENTATION_API_VERSION = 'mss.io/v1alpha1';
export const PRESENTATION_KIND = 'AdminPagePresentation';
export const MAX_PRESENTATION_DOCUMENT_BYTES = 128 * 1024;
export const PRESENTATION_PAGE_SIZE = 100;

export type PresentationScope = 'application' | 'role' | 'user';

export type PresentationJSONValue =
  | boolean
  | null
  | number
  | string
  | PresentationJSONArray
  | PresentationJSONObject;

export interface PresentationJSONArray extends ReadonlyArray<PresentationJSONValue> {}

export interface PresentationJSONObject {
  readonly [key: string]: PresentationJSONValue;
}

export interface LocalizedText {
  'en-US'?: string;
  'zh-CN'?: string;
}

export interface PresentationIssue {
  code: string;
  path: string;
  message: string;
}

export interface PresentationCapabilityField {
  id: string;
  label: LocalizedText;
  valueType: string;
  required: boolean;
  sortable: boolean;
  filterable: boolean;
  surfaces: string[];
  components: string[];
}

export interface PresentationCapability {
  pageKey: string;
  definitionVersion: string;
  definitionHash: string;
  components: string[];
  fields: PresentationCapabilityField[];
  dataSources: string[];
  actions: string[];
  defaultPresentation: PresentationJSONObject;
  definition: PageCapabilityDefinition;
}

export interface PresentationCapabilityCatalog {
  items: PresentationCapability[];
  recoveryMode: boolean;
}

export interface PresentationProfileIdentity {
  scope: PresentationScope;
  subjectID: string;
  pageKey: string;
}

export interface PresentationProfileSummary extends PresentationProfileIdentity {
  id: string;
  state: 'draft' | 'published';
  version: number;
  draftValid?: boolean;
  publishedRevision: number;
  createdBy: string;
  updatedBy: string;
  createdAt: string;
  updatedAt: string;
}

export interface PresentationDraft {
  document: PresentationJSONObject;
  digest: string;
  definitionHash: string;
  valid: boolean;
  issues: PresentationIssue[];
}

export interface PresentationRevisionSummary {
  revision: number;
  aggregateVersion: number;
  contentDigest: string;
  definitionHash: string;
  transition: 'publish' | 'rollback';
  sourceRevision?: number;
  actorID: string;
  createdAt: string;
}

export interface PresentationRevision extends PresentationRevisionSummary {
  profileID: string;
  document: PresentationJSONObject;
}

export interface PresentationProfile extends PresentationProfileSummary {
  draft?: PresentationDraft;
  published?: PresentationRevisionSummary;
}

export interface PresentationProfilePage {
  items: PresentationProfileSummary[];
  page: number;
  pageSize: number;
  total: number;
}

export interface PresentationRevisionPage {
  items: PresentationRevisionSummary[];
  page: number;
  pageSize: number;
  total: number;
}

export interface PresentationValidationResult {
  structurallyValid: boolean;
  semanticallyValid: boolean;
  canonicalDocument?: PresentationJSONObject;
  digest?: string;
  currentDefinition?: string;
  issues: PresentationIssue[];
}

export interface PresentationTransitionResult {
  profile: PresentationProfile;
  revision: PresentationRevision;
  replayed: boolean;
}

export class PresentationContractError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'PresentationContractError';
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function requiredString(value: Record<string, unknown>, key: string, max = 512): string {
  const candidate = value[key];
  if (typeof candidate !== 'string' || !candidate.trim() || [...candidate].length > max) {
    throw new PresentationContractError(`Presentation field ${key} is invalid`);
  }
  return candidate.trim();
}

function optionalString(
  value: Record<string, unknown>,
  key: string,
  max = 512,
): string | undefined {
  const candidate = value[key];
  if (candidate === undefined || candidate === null || candidate === '') return undefined;
  if (typeof candidate !== 'string' || [...candidate].length > max) {
    throw new PresentationContractError(`Presentation field ${key} is invalid`);
  }
  return candidate.trim();
}

function requiredBoolean(value: Record<string, unknown>, key: string): boolean {
  if (typeof value[key] !== 'boolean') {
    throw new PresentationContractError(`Presentation field ${key} is invalid`);
  }
  return value[key];
}

function integer(value: Record<string, unknown>, key: string, minimum: number): number {
  const candidate = value[key];
  if (typeof candidate !== 'number' || !Number.isSafeInteger(candidate) || candidate < minimum) {
    throw new PresentationContractError(`Presentation field ${key} is invalid`);
  }
  return candidate;
}

function dateString(value: Record<string, unknown>, key: string): string {
  const candidate = requiredString(value, key, 64);
  if (!Number.isFinite(Date.parse(candidate))) {
    throw new PresentationContractError(`Presentation field ${key} is invalid`);
  }
  return candidate;
}

function digest(value: Record<string, unknown>, key: string): string {
  const candidate = requiredString(value, key, 71);
  if (!/^sha256:[0-9a-f]{64}$/.test(candidate)) {
    throw new PresentationContractError(`Presentation field ${key} is invalid`);
  }
  return candidate;
}

function stringArray(value: unknown, label: string, maximum = 500): string[] {
  if (!Array.isArray(value) || value.length > maximum) {
    throw new PresentationContractError(`${label} is invalid`);
  }
  return value.map((entry) => {
    if (typeof entry !== 'string' || !entry.trim() || [...entry].length > 512) {
      throw new PresentationContractError(`${label} is invalid`);
    }
    return entry.trim();
  });
}

function parseJSONValue(value: unknown, depth = 0): PresentationJSONValue {
  if (depth > 20) throw new PresentationContractError('Presentation JSON is too deeply nested');
  if (value === null || typeof value === 'boolean' || typeof value === 'string') return value;
  if (typeof value === 'number') {
    if (!Number.isFinite(value))
      throw new PresentationContractError('Presentation JSON number is invalid');
    return value;
  }
  if (Array.isArray(value)) {
    if (value.length > 2_000)
      throw new PresentationContractError('Presentation JSON array is too large');
    return value.map((entry) => parseJSONValue(entry, depth + 1));
  }
  if (!isRecord(value) || Object.keys(value).length > 2_000) {
    throw new PresentationContractError('Presentation JSON value is invalid');
  }
  const result: Record<string, PresentationJSONValue> = Object.create(null);
  for (const [key, entry] of Object.entries(value)) {
    if (!key || key === '__proto__' || key === 'constructor' || key === 'prototype') {
      throw new PresentationContractError('Presentation JSON key is invalid');
    }
    result[key] = parseJSONValue(entry, depth + 1);
  }
  return result;
}

function parseJSONObject(value: unknown, label: string): PresentationJSONObject {
  const parsed = parseJSONValue(value);
  if (!isRecord(parsed)) throw new PresentationContractError(`${label} must be an object`);
  const encoded = JSON.stringify(parsed);
  if (new TextEncoder().encode(encoded).byteLength > MAX_PRESENTATION_DOCUMENT_BYTES) {
    throw new PresentationContractError(`${label} exceeds the encoded size limit`);
  }
  return parsed;
}

function parseLocalizedText(value: unknown): LocalizedText {
  if (!isRecord(value)) throw new PresentationContractError('Localized text is invalid');
  const zhCN = optionalString(value, 'zh-CN', 200);
  const enUS = optionalString(value, 'en-US', 200);
  if (!zhCN && !enUS) throw new PresentationContractError('Localized text is empty');
  return { ...(zhCN ? { 'zh-CN': zhCN } : {}), ...(enUS ? { 'en-US': enUS } : {}) };
}

function parseIssue(value: unknown): PresentationIssue {
  if (!isRecord(value)) throw new PresentationContractError('Presentation issue is invalid');
  return {
    code: requiredString(value, 'code', 120),
    path: requiredString(value, 'path', 512),
    message: requiredString(value, 'message', 2_000),
  };
}

function parseIssues(value: unknown): PresentationIssue[] {
  if (!Array.isArray(value) || value.length > 1_000) {
    throw new PresentationContractError('Presentation issues are invalid');
  }
  return value.map(parseIssue);
}

function parseCapability(value: unknown): PresentationCapability {
  if (!isRecord(value)) throw new PresentationContractError('Presentation capability is invalid');
  const rawComponents = value.components;
  const rawFields = value.fields;
  const rawDataSources = value.dataSources;
  const rawActions = value.actions;
  if (
    !Array.isArray(rawComponents) ||
    !Array.isArray(rawFields) ||
    !Array.isArray(rawDataSources) ||
    !Array.isArray(rawActions) ||
    rawComponents.length > 500 ||
    rawFields.length > 500 ||
    rawDataSources.length > 500 ||
    rawActions.length > 500
  ) {
    throw new PresentationContractError('Presentation capability collections are invalid');
  }
  const components = rawComponents.map((entry) => {
    if (!isRecord(entry)) throw new PresentationContractError('Presentation component is invalid');
    return requiredString(entry, 'id', 120);
  });
  const fields = rawFields.map((entry): PresentationCapabilityField => {
    if (!isRecord(entry)) throw new PresentationContractError('Presentation field is invalid');
    return {
      id: requiredString(entry, 'id', 120),
      label: parseLocalizedText(entry.label),
      valueType: requiredString(entry, 'valueType', 40),
      required: requiredBoolean(entry, 'required'),
      sortable: requiredBoolean(entry, 'sortable'),
      filterable: requiredBoolean(entry, 'filterable'),
      surfaces: stringArray(entry.surfaces, 'Presentation field surfaces', 10),
      components: stringArray(entry.components, 'Presentation field components', 100),
    };
  });
  const dataSources = rawDataSources.map((entry) => {
    if (!isRecord(entry))
      throw new PresentationContractError('Presentation data source is invalid');
    stringArray(entry.requiredPermissions, 'Presentation data source permissions', 500);
    return requiredString(entry, 'id', 120);
  });
  const actions = rawActions.map((entry) => {
    if (!isRecord(entry)) throw new PresentationContractError('Presentation action is invalid');
    stringArray(entry.requiredPermissions, 'Presentation action permissions', 500);
    stringArray(entry.placements, 'Presentation action placements', 10);
    return requiredString(entry, 'id', 120);
  });
  const unique = (items: readonly string[], label: string) => {
    if (new Set(items).size !== items.length) {
      throw new PresentationContractError(`${label} contains duplicates`);
    }
  };
  unique(components, 'Presentation components');
  unique(
    fields.map((field) => field.id),
    'Presentation fields',
  );
  unique(dataSources, 'Presentation data sources');
  unique(actions, 'Presentation actions');
  const normalizedDefinition = parseJSONValue(value);
  if (!isRecord(normalizedDefinition)) {
    throw new PresentationContractError('Presentation capability is invalid');
  }
  const definition = normalizedDefinition as unknown as PageCapabilityDefinition;
  let definitionIssues: ReturnType<typeof validatePageCapabilityDefinition>;
  try {
    definitionIssues = validatePageCapabilityDefinition(definition);
  } catch {
    throw new PresentationContractError('Presentation capability definition is invalid');
  }
  if (definitionIssues.length > 0) {
    throw new PresentationContractError(
      `Presentation capability definition is invalid: ${definitionIssues[0]?.code ?? 'unknown'}`,
    );
  }
  return {
    pageKey: requiredString(value, 'pageKey', 120),
    definitionVersion: requiredString(value, 'definitionVersion', 40),
    definitionHash: digest(value, 'definitionHash'),
    components,
    fields,
    dataSources,
    actions,
    defaultPresentation: parseJSONObject(value.defaultPresentation, 'Default presentation'),
    definition,
  };
}

export function parsePresentationCapabilityCatalog(value: unknown): PresentationCapabilityCatalog {
  if (!isRecord(value) || !Array.isArray(value.items) || value.items.length > 500) {
    throw new PresentationContractError('Presentation capability catalog is invalid');
  }
  const items = value.items.map(parseCapability);
  if (new Set(items.map((item) => item.pageKey)).size !== items.length) {
    throw new PresentationContractError('Presentation capability page keys must be unique');
  }
  return { items, recoveryMode: requiredBoolean(value, 'recoveryMode') };
}

function parseScope(value: unknown): PresentationScope {
  if (value !== 'application' && value !== 'role' && value !== 'user') {
    throw new PresentationContractError('Presentation scope is invalid');
  }
  return value;
}

export function parsePresentationProfileSummary(value: unknown): PresentationProfileSummary {
  if (!isRecord(value)) throw new PresentationContractError('Presentation profile is invalid');
  const state = value.state;
  if (state !== 'draft' && state !== 'published') {
    throw new PresentationContractError('Presentation state is invalid');
  }
  const draftValid = value.draftValid;
  if (draftValid !== undefined && typeof draftValid !== 'boolean') {
    throw new PresentationContractError('Presentation draft validity is invalid');
  }
  return {
    id: requiredString(value, 'id', 64),
    scope: parseScope(value.scope),
    subjectID: optionalString(value, 'subjectID', 160) ?? '',
    pageKey: requiredString(value, 'pageKey', 120),
    state,
    version: integer(value, 'version', 1),
    ...(typeof draftValid === 'boolean' ? { draftValid } : {}),
    publishedRevision:
      value.publishedRevision === undefined ? 0 : integer(value, 'publishedRevision', 0),
    createdBy: requiredString(value, 'createdBy', 64),
    updatedBy: requiredString(value, 'updatedBy', 64),
    createdAt: dateString(value, 'createdAt'),
    updatedAt: dateString(value, 'updatedAt'),
  };
}

function parseRevisionSummary(value: unknown): PresentationRevisionSummary {
  if (!isRecord(value)) throw new PresentationContractError('Presentation revision is invalid');
  const transition = value.transition;
  if (transition !== 'publish' && transition !== 'rollback') {
    throw new PresentationContractError('Presentation revision transition is invalid');
  }
  const sourceRevision =
    value.sourceRevision === undefined ? undefined : integer(value, 'sourceRevision', 1);
  return {
    revision: integer(value, 'revision', 1),
    aggregateVersion: integer(value, 'aggregateVersion', 1),
    contentDigest: digest(value, 'contentDigest'),
    definitionHash: digest(value, 'definitionHash'),
    transition,
    ...(sourceRevision ? { sourceRevision } : {}),
    actorID: requiredString(value, 'actorID', 64),
    createdAt: dateString(value, 'createdAt'),
  };
}

export function parsePresentationProfile(value: unknown): PresentationProfile {
  if (!isRecord(value)) throw new PresentationContractError('Presentation profile is invalid');
  const summary = parsePresentationProfileSummary(value);
  let draft: PresentationDraft | undefined;
  if (value.draft !== undefined && value.draft !== null) {
    if (!isRecord(value.draft))
      throw new PresentationContractError('Presentation draft is invalid');
    draft = {
      document: parseJSONObject(value.draft.document, 'Presentation draft document'),
      digest: digest(value.draft, 'digest'),
      definitionHash: digest(value.draft, 'definitionHash'),
      valid: requiredBoolean(value.draft, 'valid'),
      issues: parseIssues(value.draft.issues),
    };
  }
  const published =
    value.published === undefined || value.published === null
      ? undefined
      : parseRevisionSummary(value.published);
  if ((summary.state === 'draft') !== Boolean(draft)) {
    throw new PresentationContractError('Presentation profile state and draft disagree');
  }
  if (summary.publishedRevision > 0 && !published) {
    throw new PresentationContractError('Published presentation summary is missing');
  }
  return { ...summary, ...(draft ? { draft } : {}), ...(published ? { published } : {}) };
}

export function parsePresentationProfilePage(value: unknown): PresentationProfilePage {
  if (
    !isRecord(value) ||
    !Array.isArray(value.items) ||
    value.items.length > PRESENTATION_PAGE_SIZE
  ) {
    throw new PresentationContractError('Presentation profile page is invalid');
  }
  const items = value.items.map(parsePresentationProfileSummary);
  if (new Set(items.map((item) => item.id)).size !== items.length) {
    throw new PresentationContractError('Presentation profile page contains duplicate IDs');
  }
  return {
    items,
    page: integer(value, 'page', 1),
    pageSize: integer(value, 'pageSize', 1),
    total: integer(value, 'total', 0),
  };
}

export function parsePresentationRevision(value: unknown): PresentationRevision {
  if (!isRecord(value)) throw new PresentationContractError('Presentation revision is invalid');
  return {
    ...parseRevisionSummary(value),
    profileID: requiredString(value, 'profileID', 64),
    document: parseJSONObject(value.document, 'Presentation revision document'),
  };
}

export function parsePresentationRevisionPage(value: unknown): PresentationRevisionPage {
  if (
    !isRecord(value) ||
    !Array.isArray(value.items) ||
    value.items.length > PRESENTATION_PAGE_SIZE
  ) {
    throw new PresentationContractError('Presentation revision page is invalid');
  }
  const items = value.items.map(parseRevisionSummary);
  if (new Set(items.map((item) => item.revision)).size !== items.length) {
    throw new PresentationContractError('Presentation revision page contains duplicates');
  }
  return {
    items,
    page: integer(value, 'page', 1),
    pageSize: integer(value, 'pageSize', 1),
    total: integer(value, 'total', 0),
  };
}

export function parsePresentationValidation(value: unknown): PresentationValidationResult {
  if (!isRecord(value)) throw new PresentationContractError('Presentation validation is invalid');
  return {
    structurallyValid: requiredBoolean(value, 'structurallyValid'),
    semanticallyValid: requiredBoolean(value, 'semanticallyValid'),
    ...(value.canonicalDocument === undefined
      ? {}
      : { canonicalDocument: parseJSONObject(value.canonicalDocument, 'Canonical presentation') }),
    ...(optionalString(value, 'digest', 71) ? { digest: digest(value, 'digest') } : {}),
    ...(optionalString(value, 'currentDefinition', 71)
      ? { currentDefinition: digest(value, 'currentDefinition') }
      : {}),
    issues: parseIssues(value.issues),
  };
}

export function parsePresentationTransition(value: unknown): PresentationTransitionResult {
  if (!isRecord(value)) throw new PresentationContractError('Presentation transition is invalid');
  return {
    profile: parsePresentationProfile(value.profile),
    revision: parsePresentationRevision(value.revision),
    replayed: requiredBoolean(value, 'replayed'),
  };
}

export function parsePresentationDocumentText(value: string): PresentationJSONObject {
  if (new TextEncoder().encode(value).byteLength > MAX_PRESENTATION_DOCUMENT_BYTES) {
    throw new PresentationContractError('Presentation document exceeds 128 KiB');
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch {
    throw new PresentationContractError('Presentation document must be valid JSON');
  }
  return parseJSONObject(parsed, 'Presentation document');
}

export function formatPresentationDocument(value: PresentationJSONObject): string {
  return JSON.stringify(value, null, 2);
}

export function buildInitialPresentationDocument(
  capability: PresentationCapability,
  identity: PresentationProfileIdentity,
): PresentationJSONObject {
  const defaultTitle = capability.defaultPresentation.title;
  const title = isRecord(defaultTitle)
    ? parseLocalizedText(defaultTitle)
    : { 'en-US': identity.pageKey };
  const titleDocument: PresentationJSONObject = {
    ...(title['en-US'] ? { 'en-US': title['en-US'] } : {}),
    ...(title['zh-CN'] ? { 'zh-CN': title['zh-CN'] } : {}),
  };
  return {
    apiVersion: PRESENTATION_API_VERSION,
    kind: PRESENTATION_KIND,
    metadata: {
      name: `${identity.pageKey}-${identity.scope}`,
      pageKey: identity.pageKey,
      definitionHash: capability.definitionHash,
      scope: {
        kind: identity.scope,
        ...(identity.scope === 'application' ? {} : { subject: identity.subjectID }),
      },
    },
    spec: { title: titleDocument },
  };
}

export function presentationProfileETag(
  profile: Pick<PresentationProfileSummary, 'id' | 'version'>,
) {
  return JSON.stringify(`presentation-profile-${profile.id}-${profile.version}`);
}

export function presentationConflictCurrent(error: unknown): PresentationProfile | undefined {
  const failure = error as {
    data?: unknown;
    response?: { data?: unknown };
  };
  const body = failure?.data ?? failure?.response?.data;
  if (
    !isRecord(body) ||
    !isRecord(body.data) ||
    body.errorCode !== 'PRESENTATION_REVISION_CONFLICT'
  ) {
    return undefined;
  }
  try {
    return parsePresentationProfile(body.data.current);
  } catch {
    return undefined;
  }
}
