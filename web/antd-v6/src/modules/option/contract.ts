export const OPTION_PAGE_SIZES = [20, 50, 100] as const;
export const MAX_OPTION_PAGE_SIZE = 100;
export const MAX_OPTION_ITEMS = 1_000;
export const MAX_OPTION_ITEMS_SIZE = 256 * 1_024;
export const MAX_OPTION_CATEGORY = 50;
export const MAX_OPTION_NAME = 255;
export const MAX_OPTION_DISPLAY_NAME = 255;
export const MAX_OPTION_DESCRIPTION = 4_096;
export const MAX_OPTION_REMARK = 255;
export const MAX_OPTION_ITEM_KEY = 128;
export const MAX_OPTION_ITEM_LABEL = 255;
export const MAX_OPTION_ITEM_VALUE = 1_024;
export const MAX_OPTION_ITEM_COLOR = 64;
export const MAX_OPTION_ITEM_ICON = 512;
export const MAX_OPTION_ITEM_SORT = 1_000_000;
const MAX_OPTION_EXTRA_DEPTH = 8;
const MAX_OPTION_EXTRA_ENTRIES = 128;

export type OptionStatus = 'enabled' | 'disabled';
export type OptionStatusFilter = OptionStatus | 'all';
export interface OptionJSONObject {
  readonly [key: string]: OptionJSONValue;
}

export interface OptionJSONArray extends ReadonlyArray<OptionJSONValue> {}

export type OptionJSONValue = boolean | null | number | string | OptionJSONArray | OptionJSONObject;

export interface OptionItem {
  id: string;
  key: string;
  label: string;
  value: string;
  color: string;
  sort: number;
  icon?: string;
  extra?: OptionJSONObject;
}

export interface OptionItemDraft {
  id?: string;
  key: string;
  label: string;
  value: string;
  color?: string;
  sort?: number;
  icon?: string;
}

export interface OptionSummary {
  id: string;
  category: string;
  displayName: string;
  name: string;
  remark: string;
  status: OptionStatus;
  version: number;
  builtIn: boolean;
  updatedAt: string;
}

export interface OptionDetail extends OptionSummary {
  description: string;
  items: OptionItem[];
}

export interface OptionListParams {
  current: number;
  pageSize: (typeof OPTION_PAGE_SIZES)[number];
  status: OptionStatusFilter;
  category?: string;
  name?: string;
}

export interface OptionPage {
  data: OptionSummary[];
  total: number;
  current: number;
  pageSize: number;
}

export interface OptionFormValues {
  category: string;
  displayName?: string;
  description?: string;
  name: string;
  remark?: string;
  status: OptionStatus;
  items?: OptionItemDraft[];
}

export interface OptionWritePayload {
  category?: string;
  displayName: string;
  description: string;
  name?: string;
  remark: string;
  status?: OptionStatus;
  items: Array<OptionItemDraft & { extra?: OptionJSONObject }>;
}

export class OptionContractError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'OptionContractError';
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
    throw new OptionContractError(`Option field ${key} is invalid`);
  }
  return candidate.trim();
}

function optionalString(value: Record<string, unknown>, key: string, maxLength: number): string {
  const candidate = value[key];
  if (candidate === undefined || candidate === null) return '';
  if (typeof candidate !== 'string' || runeLength(candidate) > maxLength) {
    throw new OptionContractError(`Option field ${key} is invalid`);
  }
  return candidate.trim();
}

function positiveInteger(value: Record<string, unknown>, key: string): number {
  const candidate = value[key];
  if (typeof candidate !== 'number' || !Number.isSafeInteger(candidate) || candidate <= 0) {
    throw new OptionContractError(`Option field ${key} is invalid`);
  }
  return candidate;
}

function nonNegativeInteger(value: Record<string, unknown>, key: string): number {
  const candidate = value[key];
  if (typeof candidate !== 'number' || !Number.isSafeInteger(candidate) || candidate < 0) {
    throw new OptionContractError(`Option field ${key} is invalid`);
  }
  return candidate;
}

function statusField(value: Record<string, unknown>): OptionStatus {
  if (value.status !== 'enabled' && value.status !== 'disabled') {
    throw new OptionContractError('Option field status is invalid');
  }
  return value.status;
}

function dateField(value: Record<string, unknown>, key: string): string {
  const candidate = requiredString(value, key, 64);
  if (!Number.isFinite(Date.parse(candidate))) {
    throw new OptionContractError(`Option field ${key} is invalid`);
  }
  return candidate;
}

function normalizeJSONValue(value: unknown, depth = 0): OptionJSONValue {
  if (depth > MAX_OPTION_EXTRA_DEPTH) {
    throw new OptionContractError('Option item extra is too deeply nested');
  }
  if (value === null || typeof value === 'boolean' || typeof value === 'string') return value;
  if (typeof value === 'number') {
    if (!Number.isFinite(value))
      throw new OptionContractError('Option item extra number is invalid');
    return value;
  }
  if (Array.isArray(value)) {
    if (value.length > MAX_OPTION_EXTRA_ENTRIES) {
      throw new OptionContractError('Option item extra array is too large');
    }
    return value.map((entry) => normalizeJSONValue(entry, depth + 1));
  }
  if (!isRecord(value) || Object.keys(value).length > MAX_OPTION_EXTRA_ENTRIES) {
    throw new OptionContractError('Option item extra is invalid');
  }
  const result: Record<string, OptionJSONValue> = Object.create(null);
  for (const [key, entry] of Object.entries(value)) {
    if (
      !key ||
      runeLength(key) > MAX_OPTION_ITEM_KEY ||
      key === '__proto__' ||
      key === 'constructor' ||
      key === 'prototype'
    ) {
      throw new OptionContractError('Option item extra key is invalid');
    }
    result[key] = normalizeJSONValue(entry, depth + 1);
  }
  return Object.freeze(result);
}

function normalizeItem(
  value: unknown,
  index: number,
  requireID: boolean,
): OptionItemDraft & {
  extra?: OptionJSONObject;
} {
  if (!isRecord(value)) throw new OptionContractError(`Option item ${index} is invalid`);
  const rawID = value.id;
  let id: string | undefined;
  if (rawID !== undefined && rawID !== null && rawID !== '') {
    if (typeof rawID !== 'string' || !rawID.trim() || runeLength(rawID) > 64) {
      throw new OptionContractError(`Option item ${index} id is invalid`);
    }
    id = rawID.trim();
  } else if (requireID) {
    throw new OptionContractError(`Option item ${index} id is missing`);
  }
  const rawSort = value.sort ?? 0;
  if (
    typeof rawSort !== 'number' ||
    !Number.isSafeInteger(rawSort) ||
    rawSort < -MAX_OPTION_ITEM_SORT ||
    rawSort > MAX_OPTION_ITEM_SORT
  ) {
    throw new OptionContractError(`Option item ${index} sort is invalid`);
  }
  const extraValue = value.extra;
  const extra =
    extraValue === undefined || extraValue === null ? undefined : normalizeJSONValue(extraValue);
  if (extra !== undefined && (Array.isArray(extra) || !isRecord(extra))) {
    throw new OptionContractError(`Option item ${index} extra is invalid`);
  }
  return {
    ...(id ? { id } : {}),
    key: requiredString(value, 'key', MAX_OPTION_ITEM_KEY),
    label: requiredString(value, 'label', MAX_OPTION_ITEM_LABEL),
    value: requiredString(value, 'value', MAX_OPTION_ITEM_VALUE),
    color: optionalString(value, 'color', MAX_OPTION_ITEM_COLOR),
    sort: rawSort,
    ...(optionalString(value, 'icon', MAX_OPTION_ITEM_ICON)
      ? { icon: optionalString(value, 'icon', MAX_OPTION_ITEM_ICON) }
      : {}),
    ...(extra ? { extra: extra as OptionJSONObject } : {}),
  };
}

function validateItems(
  value: unknown,
  requireID: boolean,
): Array<OptionItemDraft & { extra?: OptionJSONObject }> {
  if (value === undefined || value === null) return [];
  if (!Array.isArray(value) || value.length > MAX_OPTION_ITEMS) {
    throw new OptionContractError('Option items are invalid');
  }
  const items = value.map((item, index) => normalizeItem(item, index, requireID));
  const ids = new Set<string>();
  const keys = new Set<string>();
  const values = new Set<string>();
  for (const item of items) {
    if (item.id) {
      if (ids.has(item.id)) throw new OptionContractError('Option item IDs must be unique');
      ids.add(item.id);
    }
    if (keys.has(item.key)) throw new OptionContractError('Option item keys must be unique');
    if (values.has(item.value)) throw new OptionContractError('Option item values must be unique');
    keys.add(item.key);
    values.add(item.value);
  }
  return items;
}

function validateEncodedItems(items: readonly unknown[]): void {
  if (new TextEncoder().encode(JSON.stringify(items)).byteLength > MAX_OPTION_ITEMS_SIZE) {
    throw new OptionContractError('Option items exceed the encoded size limit');
  }
}

export function parseOptionSummary(value: unknown): OptionSummary {
  if (!isRecord(value)) throw new OptionContractError('Option must be an object');
  if (typeof value.builtIn !== 'boolean') {
    throw new OptionContractError('Option field builtIn is invalid');
  }
  return {
    id: requiredString(value, 'id', 64),
    category: requiredString(value, 'category', MAX_OPTION_CATEGORY),
    displayName: optionalString(value, 'displayName', MAX_OPTION_DISPLAY_NAME),
    name: requiredString(value, 'name', MAX_OPTION_NAME),
    remark: optionalString(value, 'remark', MAX_OPTION_REMARK),
    status: statusField(value),
    version: positiveInteger(value, 'version'),
    builtIn: value.builtIn,
    updatedAt: dateField(value, 'updatedAt'),
  };
}

export function parseOptionDetail(value: unknown): OptionDetail {
  if (!isRecord(value)) throw new OptionContractError('Option must be an object');
  const items = validateItems(value.items, true) as OptionItem[];
  validateEncodedItems(items);
  return {
    ...parseOptionSummary(value),
    description: optionalString(value, 'description', MAX_OPTION_DESCRIPTION),
    items,
  };
}

export function parseOptionPage(
  value: unknown,
  expected?: Pick<OptionListParams, 'current' | 'pageSize'>,
): OptionPage {
  if (!isRecord(value) || !Array.isArray(value.data)) {
    throw new OptionContractError('Option page is invalid');
  }
  const data = value.data.map(parseOptionSummary);
  const total = nonNegativeInteger(value, 'total');
  const current = positiveInteger(value, 'current');
  const pageSize = positiveInteger(value, 'pageSize');
  if (
    pageSize > MAX_OPTION_PAGE_SIZE ||
    data.length > pageSize ||
    data.length > total ||
    new Set(data.map((option) => option.id)).size !== data.length ||
    (expected && (current !== expected.current || pageSize !== expected.pageSize))
  ) {
    throw new OptionContractError('Option page metadata is invalid');
  }
  return { data, total, current, pageSize };
}

export interface SerializeOptionWriteOptions {
  base?: OptionDetail;
}

export function serializeOptionWrite(
  values: OptionFormValues,
  options: SerializeOptionWriteOptions = {},
): OptionWritePayload {
  if (!isRecord(values)) throw new OptionContractError('Option form is invalid');
  const base = options.base;
  const normalizedItems = validateItems(values.items, false);
  const baseItems = new Map(base?.items.map((item) => [item.id, item]) ?? []);
  const items = normalizedItems.map((item) => {
    const baseItem = item.id ? baseItems.get(item.id) : undefined;
    const result: OptionItemDraft & { extra?: OptionJSONObject } = {
      ...(base && item.id ? { id: item.id } : {}),
      key: item.key,
      label: item.label,
      value: item.value,
      color: item.color ?? '',
      sort: item.sort ?? 0,
      ...(item.icon ? { icon: item.icon } : {}),
    };
    if (baseItem?.extra) result.extra = baseItem.extra;
    return result;
  });
  // Reserve the 32-byte server ID cost for every new item before sending.
  validateEncodedItems(items.map((item) => (item.id ? item : { ...item, id: '0'.repeat(32) })));

  const payload: OptionWritePayload = {
    displayName: optionalString(values, 'displayName', MAX_OPTION_DISPLAY_NAME),
    description: optionalString(values, 'description', MAX_OPTION_DESCRIPTION),
    remark: optionalString(values, 'remark', MAX_OPTION_REMARK),
    items,
  };
  if (base?.builtIn) {
    return payload;
  }
  payload.category = requiredString(values, 'category', MAX_OPTION_CATEGORY);
  payload.name = requiredString(values, 'name', MAX_OPTION_NAME);
  payload.status = statusField(values);
  return payload;
}

export function optionRevisionETag(option: Pick<OptionSummary, 'id' | 'version'>): string {
  if (!option.id || !Number.isSafeInteger(option.version) || option.version <= 0) {
    throw new OptionContractError('Option revision is invalid');
  }
  return JSON.stringify(`option-${option.id}-v${option.version}`);
}

export function isOptionPageSize(value: number): value is OptionListParams['pageSize'] {
  return (OPTION_PAGE_SIZES as readonly number[]).includes(value);
}
