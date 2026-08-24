import type { PageCapabilityDefinition } from '@mss-admin-core/shared/presentation/contract';

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

const invalidResponse = () => new PresentationContractError('Invalid presentation response');

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function requiredString(value: Record<string, unknown>, key: string, max = 512): string {
  const candidate = value[key];
  if (typeof candidate !== 'string' || !candidate.trim() || [...candidate].length > max) {
    throw invalidResponse();
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
    throw invalidResponse();
  }
  return candidate.trim();
}

function requiredBoolean(value: Record<string, unknown>, key: string): boolean {
  if (typeof value[key] !== 'boolean') {
    throw invalidResponse();
  }
  return value[key];
}

function integer(value: Record<string, unknown>, key: string, minimum: number): number {
  const candidate = value[key];
  if (typeof candidate !== 'number' || !Number.isSafeInteger(candidate) || candidate < minimum) {
    throw invalidResponse();
  }
  return candidate;
}

function dateString(value: Record<string, unknown>, key: string): string {
  const candidate = requiredString(value, key, 64);
  if (!Number.isFinite(Date.parse(candidate))) {
    throw invalidResponse();
  }
  return candidate;
}

function digest(value: Record<string, unknown>, key: string): string {
  const candidate = requiredString(value, key, 71);
  if (!/^sha256:[0-9a-f]{64}$/.test(candidate)) {
    throw invalidResponse();
  }
  return candidate;
}

function stringArray(value: unknown, maximum = 500): string[] {
  if (!Array.isArray(value) || value.length > maximum) {
    throw invalidResponse();
  }
  return value.map((entry) => {
    if (typeof entry !== 'string' || !entry.trim() || [...entry].length > 512) {
      throw invalidResponse();
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

function parseJSONObject(value: unknown): PresentationJSONObject {
  const parsed = parseJSONValue(value);
  if (!isRecord(parsed)) throw invalidResponse();
  const encoded = JSON.stringify(parsed);
  if (new TextEncoder().encode(encoded).byteLength > MAX_PRESENTATION_DOCUMENT_BYTES) {
    throw invalidResponse();
  }
  return parsed;
}

function parseLocalizedText(value: unknown): LocalizedText {
  if (!isRecord(value)) throw invalidResponse();
  const zhCN = optionalString(value, 'zh-CN', 200);
  const enUS = optionalString(value, 'en-US', 200);
  if (!zhCN && !enUS) throw invalidResponse();
  return { ...(zhCN ? { 'zh-CN': zhCN } : {}), ...(enUS ? { 'en-US': enUS } : {}) };
}

function parseIssue(value: unknown): PresentationIssue {
  if (!isRecord(value)) throw invalidResponse();
  return {
    code: requiredString(value, 'code', 120),
    path: requiredString(value, 'path', 512),
    message: requiredString(value, 'message', 2_000),
  };
}

function parseIssues(value: unknown): PresentationIssue[] {
  if (!Array.isArray(value) || value.length > 1_000) {
    throw invalidResponse();
  }
  return value.map(parseIssue);
}

function recordArray(value: unknown, maximum: number): Record<string, unknown>[] {
  if (!Array.isArray(value) || value.length > maximum) {
    throw invalidResponse();
  }
  return value.map((entry) => {
    if (!isRecord(entry)) throw invalidResponse();
    return entry;
  });
}

function validateCapabilityDefaults(value: unknown) {
  if (!isRecord(value)) throw invalidResponse();
  parseLocalizedText(value.title);
  requiredString(value, 'dataSource', 120);
  const list = value.list;
  const search = value.search;
  const form = value.form;
  const detail = value.detail;
  if (!isRecord(list) || !isRecord(search) || !isRecord(form) || !isRecord(detail)) {
    throw invalidResponse();
  }
  const fields = [
    ...recordArray(list.columns, 100),
    ...recordArray(search.fields, 100),
    ...recordArray(form.fields, 100),
    ...recordArray(detail.fields, 100),
  ];
  for (const field of fields) {
    requiredString(field, 'field', 120);
    requiredString(field, 'component', 120);
    integer(field, 'order', 0);
    requiredBoolean(field, 'hidden');
  }
  const density = requiredString(list, 'density', 20);
  if (density !== 'compact' && density !== 'middle' && density !== 'large') {
    throw invalidResponse();
  }
  integer(list, 'pageSize', 1);
  for (const sort of recordArray(list.defaultSort, 3)) {
    requiredString(sort, 'field', 120);
    const direction = requiredString(sort, 'direction', 4);
    if (direction !== 'asc' && direction !== 'desc') {
      throw invalidResponse();
    }
  }
  requiredBoolean(search, 'collapsedByDefault');
  integer(form, 'columns', 1);
  integer(detail, 'columns', 1);
  for (const action of recordArray(value.actions, 64)) {
    requiredString(action, 'action', 120);
    requiredString(action, 'placement', 20);
    integer(action, 'order', 0);
    requiredBoolean(action, 'hidden');
  }
}

function parseCapability(value: unknown): PresentationCapability {
  if (!isRecord(value)) throw invalidResponse();
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
    throw invalidResponse();
  }
  const components = rawComponents.map((entry) => {
    if (!isRecord(entry)) throw invalidResponse();
    return requiredString(entry, 'id', 120);
  });
  const fields = rawFields.map((entry): PresentationCapabilityField => {
    if (!isRecord(entry)) throw invalidResponse();
    return {
      id: requiredString(entry, 'id', 120),
      label: parseLocalizedText(entry.label),
      valueType: requiredString(entry, 'valueType', 40),
      required: requiredBoolean(entry, 'required'),
      sortable: requiredBoolean(entry, 'sortable'),
      filterable: requiredBoolean(entry, 'filterable'),
      surfaces: stringArray(entry.surfaces, 10),
      components: stringArray(entry.components, 100),
    };
  });
  const dataSources = rawDataSources.map((entry) => {
    if (!isRecord(entry)) throw invalidResponse();
    stringArray(entry.requiredPermissions, 500);
    return requiredString(entry, 'id', 120);
  });
  const actions = rawActions.map((entry) => {
    if (!isRecord(entry)) throw invalidResponse();
    stringArray(entry.requiredPermissions, 500);
    stringArray(entry.placements, 10);
    return requiredString(entry, 'id', 120);
  });
  const unique = (items: readonly string[]) => {
    if (new Set(items).size !== items.length) {
      throw invalidResponse();
    }
  };
  unique(components);
  unique(fields.map((field) => field.id));
  unique(dataSources);
  unique(actions);
  const normalizedDefinition = parseJSONValue(value);
  if (!isRecord(normalizedDefinition)) {
    throw invalidResponse();
  }
  validateCapabilityDefaults(value.defaultPresentation);
  const definition = normalizedDefinition as unknown as PageCapabilityDefinition;
  return {
    pageKey: requiredString(value, 'pageKey', 120),
    definitionVersion: requiredString(value, 'definitionVersion', 40),
    definitionHash: digest(value, 'definitionHash'),
    components,
    fields,
    dataSources,
    actions,
    defaultPresentation: parseJSONObject(value.defaultPresentation),
    definition,
  };
}

export function parsePresentationCapabilityCatalog(value: unknown): PresentationCapabilityCatalog {
  if (!isRecord(value) || !Array.isArray(value.items) || value.items.length > 500) {
    throw invalidResponse();
  }
  const items = value.items.map(parseCapability);
  if (new Set(items.map((item) => item.pageKey)).size !== items.length) {
    throw invalidResponse();
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
      document: parseJSONObject(value.draft.document),
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
    document: parseJSONObject(value.document),
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
      : { canonicalDocument: parseJSONObject(value.canonicalDocument) }),
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
  return parseJSONObject(parsed);
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

export function presentationConflictVersion(error: unknown): number | undefined {
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
    if (!isRecord(body.data.current)) return undefined;
    return integer(body.data.current, 'version', 1);
  } catch {
    return undefined;
  }
}
