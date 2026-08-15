export const LANGUAGE_PAGE_SIZES = [20, 50, 100] as const;
export const MAX_LANGUAGE_PAGE_SIZE = 100;
export const MAX_LANGUAGE_DEFINITIONS = 1_000;
export const MAX_LANGUAGE_DEFINITIONS_SIZE = 256 * 1_024;
export const MAX_LANGUAGE_DEFINE_GROUP = 64;
export const MAX_LANGUAGE_DEFINE_KEY = 256;
export const MAX_LANGUAGE_DEFINE_VALUE = 4_096;

export type LanguageStatus = 'enabled' | 'disabled';
export type LanguageStatusFilter = LanguageStatus | 'all';
export type SupportedRuntimeLocale = 'en-US' | 'zh-CN';
export type LanguageProfile = Partial<
  Readonly<Record<SupportedRuntimeLocale, Readonly<Record<string, string>>>>
>;

export const SUPPORTED_RUNTIME_LOCALES = ['en-US', 'zh-CN'] as const;

export interface LanguageDefinition {
  id: string;
  group: string;
  key: string;
  value: string;
}

export interface LanguageDefinitionDraft {
  id?: string;
  group: string;
  key: string;
  value: string;
}

export interface LanguageSummary {
  id: string;
  name: string;
  remark: string;
  status: LanguageStatus;
  updatedAt: string;
}

export interface LanguageDetail extends LanguageSummary {
  defines: LanguageDefinition[];
}

export interface LanguageListParams {
  current: number;
  pageSize: (typeof LANGUAGE_PAGE_SIZES)[number];
  status: LanguageStatusFilter;
  name?: string;
}

export interface LanguagePage {
  data: LanguageSummary[];
  total: number;
  current: number;
  pageSize: number;
}

export interface LanguageFormValues {
  name: string;
  remark?: string;
  status: LanguageStatus;
  defines?: LanguageDefinitionDraft[];
}

export interface LanguageWritePayload {
  name: string;
  remark: string;
  status: LanguageStatus;
  defines: LanguageDefinitionDraft[];
  expectedUpdatedAt?: string;
}

export class LanguageContractError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'LanguageContractError';
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function runeLength(value: string): number {
  return [...value].length;
}

function requiredString(value: Record<string, unknown>, key: string, maxLength: number): string {
  const candidate = value[key];
  if (typeof candidate !== 'string' || !candidate.trim() || runeLength(candidate) > maxLength) {
    throw new LanguageContractError(`Language field ${key} is invalid`);
  }
  return candidate.trim();
}

function optionalString(value: Record<string, unknown>, key: string, maxLength: number): string {
  const candidate = value[key];
  if (candidate === undefined || candidate === null) return '';
  if (typeof candidate !== 'string' || runeLength(candidate) > maxLength) {
    throw new LanguageContractError(`Language field ${key} is invalid`);
  }
  return candidate;
}

function statusField(value: Record<string, unknown>): LanguageStatus {
  if (value.status !== 'enabled' && value.status !== 'disabled') {
    throw new LanguageContractError('Language field status is invalid');
  }
  return value.status;
}

function dateField(value: Record<string, unknown>, key: string): string {
  const candidate = requiredString(value, key, 64);
  if (!Number.isFinite(Date.parse(candidate))) {
    throw new LanguageContractError(`Language field ${key} is invalid`);
  }
  return candidate;
}

function integerField(value: Record<string, unknown>, key: string, min: number): number {
  const candidate = value[key];
  if (typeof candidate !== 'number' || !Number.isSafeInteger(candidate) || candidate < min) {
    throw new LanguageContractError(`Language field ${key} is invalid`);
  }
  return candidate;
}

function validateEncodedDefinitions(definitions: readonly LanguageDefinitionDraft[]): void {
  const encoded = new TextEncoder().encode(JSON.stringify(definitions));
  if (encoded.byteLength > MAX_LANGUAGE_DEFINITIONS_SIZE) {
    throw new LanguageContractError('Language definitions exceed the encoded size limit');
  }
}

export function normalizeLanguageName(value: string): string {
  const candidate = value.trim();
  if (!candidate || runeLength(candidate) > 255) {
    throw new LanguageContractError('Language field name is invalid');
  }
  try {
    const [canonical] = Intl.getCanonicalLocales(candidate);
    if (!canonical || canonical === 'und') {
      throw new LanguageContractError('Language field name is invalid');
    }
    return canonical;
  } catch (error) {
    if (error instanceof LanguageContractError) throw error;
    throw new LanguageContractError('Language field name is invalid');
  }
}

function normalizeDefinition(
  value: unknown,
  index: number,
  requireID: boolean,
): LanguageDefinitionDraft {
  if (!isRecord(value)) {
    throw new LanguageContractError(`Language definition ${index} is invalid`);
  }
  const idValue = value.id;
  let id: string | undefined;
  if (idValue !== undefined && idValue !== null && idValue !== '') {
    if (typeof idValue !== 'string' || !idValue.trim() || runeLength(idValue) > 64) {
      throw new LanguageContractError(`Language definition ${index} id is invalid`);
    }
    id = idValue.trim();
  } else if (requireID) {
    throw new LanguageContractError(`Language definition ${index} id is missing`);
  }
  const group = requiredString(value, 'group', MAX_LANGUAGE_DEFINE_GROUP);
  const key = requiredString(value, 'key', MAX_LANGUAGE_DEFINE_KEY);
  const rawValue = value.value;
  if (typeof rawValue !== 'string' || runeLength(rawValue) > MAX_LANGUAGE_DEFINE_VALUE) {
    throw new LanguageContractError(`Language definition ${index} value is invalid`);
  }
  return id ? { id, group, key, value: rawValue } : { group, key, value: rawValue };
}

function normalizeDefinitions(value: unknown, requireID: boolean): LanguageDefinitionDraft[] {
  if (value === undefined || value === null) return [];
  if (!Array.isArray(value) || value.length > MAX_LANGUAGE_DEFINITIONS) {
    throw new LanguageContractError('Language definitions are invalid');
  }
  const definitions = value.map((definition, index) =>
    normalizeDefinition(definition, index, requireID),
  );
  const ids = new Set<string>();
  const keys = new Set<string>();
  for (const definition of definitions) {
    if (definition.id) {
      if (ids.has(definition.id)) {
        throw new LanguageContractError('Language definition IDs must be unique');
      }
      ids.add(definition.id);
    }
    const compoundKey = `${definition.group}\u0000${definition.key}`;
    if (keys.has(compoundKey)) {
      throw new LanguageContractError('Language definition group and key must be unique');
    }
    keys.add(compoundKey);
  }
  return definitions;
}

export function parseLanguageSummary(value: unknown): LanguageSummary {
  if (!isRecord(value)) throw new LanguageContractError('Language must be an object');
  const name = normalizeLanguageName(requiredString(value, 'name', 255));
  return {
    id: requiredString(value, 'id', 64),
    name,
    remark: optionalString(value, 'remark', 255),
    status: statusField(value),
    updatedAt: dateField(value, 'updatedAt'),
  };
}

export function parseLanguageDetail(value: unknown): LanguageDetail {
  if (!isRecord(value)) throw new LanguageContractError('Language must be an object');
  const summary = parseLanguageSummary(value);
  const defines = normalizeDefinitions(value.defines, true) as LanguageDefinition[];
  validateEncodedDefinitions(defines);
  return {
    ...summary,
    defines,
  };
}

export function parseLanguagePage(
  value: unknown,
  expected?: Pick<LanguageListParams, 'current' | 'pageSize'>,
): LanguagePage {
  if (!isRecord(value) || !Array.isArray(value.data)) {
    throw new LanguageContractError('Language page is invalid');
  }
  const data = value.data.map(parseLanguageSummary);
  const total = integerField(value, 'total', 0);
  const current = integerField(value, 'current', 1);
  const pageSize = integerField(value, 'pageSize', 1);
  if (
    pageSize > MAX_LANGUAGE_PAGE_SIZE ||
    data.length > pageSize ||
    data.length > total ||
    new Set(data.map((language) => language.id)).size !== data.length ||
    (expected && (current !== expected.current || pageSize !== expected.pageSize))
  ) {
    throw new LanguageContractError('Language page metadata is invalid');
  }
  return { data, total, current, pageSize };
}

export function serializeLanguageWrite(
  values: LanguageFormValues,
  expectedUpdatedAt?: string,
): LanguageWritePayload {
  if (!isRecord(values)) throw new LanguageContractError('Language form is invalid');
  const name = normalizeLanguageName(requiredString(values, 'name', 255));
  const normalizedDefinitions = normalizeDefinitions(values.defines, false);
  const definitions: LanguageDefinitionDraft[] = expectedUpdatedAt
    ? normalizedDefinitions
    : normalizedDefinitions.map(({ group, key, value }) => ({ group, key, value }));
  // New definition IDs are 32 characters and are assigned by the server.
  // Reserve that response/storage cost before sending the write so a payload
  // cannot pass the browser limit and then fail only after ID generation.
  validateEncodedDefinitions(
    definitions.map((definition) =>
      definition.id ? definition : { ...definition, id: '0'.repeat(32) },
    ),
  );
  const payload: LanguageWritePayload = {
    name,
    remark: optionalString(values, 'remark', 255).trim(),
    status: statusField(values),
    defines: definitions,
  };
  if (expectedUpdatedAt !== undefined) {
    if (!Number.isFinite(Date.parse(expectedUpdatedAt))) {
      throw new LanguageContractError('Language revision is invalid');
    }
    payload.expectedUpdatedAt = expectedUpdatedAt;
  }
  return payload;
}

export function isLanguagePageSize(value: number): value is LanguageListParams['pageSize'] {
  return (LANGUAGE_PAGE_SIZES as readonly number[]).includes(value);
}

export function parseLanguageProfile(value: unknown): LanguageProfile {
  if (!isRecord(value) || Object.keys(value).length > 64) {
    throw new LanguageContractError('Language profile is invalid');
  }
  const result: Partial<Record<SupportedRuntimeLocale, Readonly<Record<string, string>>>> = {};
  for (const locale of SUPPORTED_RUNTIME_LOCALES) {
    const candidate = value[locale];
    if (candidate === undefined) continue;
    if (!isRecord(candidate) || Object.keys(candidate).length > MAX_LANGUAGE_DEFINITIONS) {
      throw new LanguageContractError(`Language profile ${locale} is invalid`);
    }
    const messages: Record<string, string> = Object.create(null);
    for (const [key, translated] of Object.entries(candidate)) {
      if (
        !key.trim() ||
        runeLength(key) > MAX_LANGUAGE_DEFINE_GROUP + MAX_LANGUAGE_DEFINE_KEY + 1 ||
        typeof translated !== 'string' ||
        runeLength(translated) > MAX_LANGUAGE_DEFINE_VALUE
      ) {
        throw new LanguageContractError(`Language profile ${locale} is invalid`);
      }
      messages[key] = translated;
    }
    const encoded = new TextEncoder().encode(JSON.stringify(messages));
    if (encoded.byteLength > MAX_LANGUAGE_DEFINITIONS_SIZE) {
      throw new LanguageContractError(`Language profile ${locale} is too large`);
    }
    result[locale] = Object.freeze(messages);
  }
  return Object.freeze(result);
}
