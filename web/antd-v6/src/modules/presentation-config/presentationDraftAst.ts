import type {
  PresentationCondition,
  PresentationSurface,
} from '@mss-admin-core/shared/presentation/contract';
import {
  formatPresentationDocument,
  PresentationContractError,
  type PresentationJSONObject,
  type PresentationJSONValue,
  parsePresentationDocumentText,
} from './contract';

export type PresentationDraftAST = PresentationJSONObject;
export type PresentationDraftLocale = 'en-US' | 'zh-CN';
export type PresentationFieldSurface = PresentationSurface;

interface MutableJSONObject {
  [key: string]: MutableJSONValue;
}
type MutableJSONValue = boolean | null | number | string | MutableJSONValue[] | MutableJSONObject;

const fieldCollectionBySurface: Readonly<Record<PresentationFieldSurface, 'columns' | 'fields'>> = {
  list: 'columns',
  search: 'fields',
  form: 'fields',
  detail: 'fields',
};

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function cloneJSON(value: PresentationJSONValue): MutableJSONValue {
  if (Array.isArray(value)) return value.map((entry) => cloneJSON(entry));
  if (isObject(value)) {
    const result: MutableJSONObject = Object.create(null);
    for (const [key, entry] of Object.entries(value)) {
      result[key] = cloneJSON(entry as PresentationJSONValue);
    }
    return result;
  }
  return value as boolean | null | number | string;
}

function mutableDocument(document: PresentationDraftAST): MutableJSONObject {
  return cloneJSON(document) as MutableJSONObject;
}

function ensureObject(parent: MutableJSONObject, key: string): MutableJSONObject {
  const current = parent[key];
  if (isObject(current)) return current as MutableJSONObject;
  const created: MutableJSONObject = Object.create(null);
  parent[key] = created;
  return created;
}

function isEmptyObject(value: MutableJSONObject): boolean {
  return Object.keys(value).length === 0;
}

function cloneForWrite(value: PresentationJSONValue): MutableJSONValue {
  return cloneJSON(value);
}

export function parsePresentationDraftAST(value: string): PresentationDraftAST {
  const document = parsePresentationDocumentText(value);
  if (!isObject(document.spec)) {
    throw new PresentationContractError('Presentation document spec must be an object');
  }
  return document;
}

export function formatPresentationDraftAST(document: PresentationDraftAST): string {
  return formatPresentationDocument(document);
}

export function presentationDraftSpec(
  document: PresentationDraftAST,
): Readonly<Record<string, unknown>> {
  return isObject(document.spec) ? document.spec : Object.freeze({});
}

export function presentationDraftSection(
  document: PresentationDraftAST,
  section: PresentationFieldSurface,
): Readonly<Record<string, unknown>> {
  const value = presentationDraftSpec(document)[section];
  return isObject(value) ? value : Object.freeze({});
}

export function setPresentationSpecOverride(
  document: PresentationDraftAST,
  section: PresentationFieldSurface | undefined,
  property: string,
  value: PresentationJSONValue | undefined,
): PresentationDraftAST {
  const next = mutableDocument(document);
  const spec = ensureObject(next, 'spec');
  const target = section ? ensureObject(spec, section) : spec;
  if (value === undefined) {
    delete target[property];
  } else {
    target[property] = cloneForWrite(value);
  }
  if (section && isEmptyObject(target)) delete spec[section];
  return next as PresentationDraftAST;
}

export function setPresentationLocalizedOverride(
  document: PresentationDraftAST,
  section: PresentationFieldSurface | undefined,
  property: string,
  locale: PresentationDraftLocale,
  value: string | undefined,
): PresentationDraftAST {
  const next = mutableDocument(document);
  const spec = ensureObject(next, 'spec');
  const target = section ? ensureObject(spec, section) : spec;
  const localized = ensureObject(target, property);
  const normalized = value?.trim();
  if (normalized) localized[locale] = normalized;
  else delete localized[locale];
  if (isEmptyObject(localized)) delete target[property];
  if (section && isEmptyObject(target)) delete spec[section];
  return next as PresentationDraftAST;
}

function mutableFieldCollection(
  document: PresentationDraftAST,
  surface: PresentationFieldSurface,
): {
  next: MutableJSONObject;
  spec: MutableJSONObject;
  section: MutableJSONObject;
  collectionKey: 'columns' | 'fields';
  collection: MutableJSONValue[];
} {
  const next = mutableDocument(document);
  const spec = ensureObject(next, 'spec');
  const section = ensureObject(spec, surface);
  const collectionKey = fieldCollectionBySurface[surface];
  const current = section[collectionKey];
  const collection = Array.isArray(current) ? current : [];
  section[collectionKey] = collection;
  return { next, spec, section, collectionKey, collection };
}

function findPatchIndex(
  collection: readonly MutableJSONValue[],
  identifierKey: 'action' | 'field',
  identifier: string,
): number {
  return collection.findIndex((entry) => isObject(entry) && entry[identifierKey] === identifier);
}

function cleanupFieldCollection(
  spec: MutableJSONObject,
  section: MutableJSONObject,
  surface: PresentationFieldSurface,
  collectionKey: 'columns' | 'fields',
  collection: MutableJSONValue[],
) {
  if (collection.length === 0) delete section[collectionKey];
  if (isEmptyObject(section)) delete spec[surface];
}

function mutateFieldPatch(
  document: PresentationDraftAST,
  surface: PresentationFieldSurface,
  field: string,
  mutate: (patch: MutableJSONObject) => void,
): PresentationDraftAST {
  const { next, spec, section, collectionKey, collection } = mutableFieldCollection(
    document,
    surface,
  );
  const existingIndex = findPatchIndex(collection, 'field', field);
  const patch =
    existingIndex >= 0 && isObject(collection[existingIndex])
      ? (collection[existingIndex] as MutableJSONObject)
      : ({ field } as MutableJSONObject);
  mutate(patch);
  if (Object.keys(patch).length === 1 && patch.field === field) {
    if (existingIndex >= 0) collection.splice(existingIndex, 1);
  } else if (existingIndex < 0) {
    collection.push(patch);
  }
  cleanupFieldCollection(spec, section, surface, collectionKey, collection);
  return next as PresentationDraftAST;
}

export function presentationFieldOverride(
  document: PresentationDraftAST,
  surface: PresentationFieldSurface,
  field: string,
): Readonly<Record<string, unknown>> | undefined {
  const section = presentationDraftSection(document, surface);
  const collection = section[fieldCollectionBySurface[surface]];
  if (!Array.isArray(collection)) return undefined;
  const patch = collection.find((entry) => isObject(entry) && entry.field === field);
  return isObject(patch) ? patch : undefined;
}

export function setPresentationFieldOverride(
  document: PresentationDraftAST,
  surface: PresentationFieldSurface,
  field: string,
  property: 'component' | 'hidden' | 'order' | 'span' | 'width',
  value: boolean | number | string | undefined,
): PresentationDraftAST {
  return mutateFieldPatch(document, surface, field, (patch) => {
    if (value === undefined) delete patch[property];
    else patch[property] = value;
  });
}

export function setPresentationFieldLocalizedOverride(
  document: PresentationDraftAST,
  surface: PresentationFieldSurface,
  field: string,
  property: 'help' | 'label' | 'placeholder',
  locale: PresentationDraftLocale,
  value: string | undefined,
): PresentationDraftAST {
  return mutateFieldPatch(document, surface, field, (patch) => {
    const localized = ensureObject(patch, property);
    const normalized = value?.trim();
    if (normalized) localized[locale] = normalized;
    else delete localized[locale];
    if (isEmptyObject(localized)) delete patch[property];
  });
}

export function setPresentationFieldCondition(
  document: PresentationDraftAST,
  surface: PresentationFieldSurface,
  field: string,
  condition: PresentationCondition | undefined,
): PresentationDraftAST {
  return mutateFieldPatch(document, surface, field, (patch) => {
    if (condition === undefined) delete patch.visibleWhen;
    else {
      patch.visibleWhen = cloneForWrite(condition as unknown as PresentationJSONValue);
    }
  });
}

export function resetPresentationFieldOverrides(
  document: PresentationDraftAST,
  surface: PresentationFieldSurface,
  field: string,
): PresentationDraftAST {
  const { next, spec, section, collectionKey, collection } = mutableFieldCollection(
    document,
    surface,
  );
  const index = findPatchIndex(collection, 'field', field);
  if (index >= 0) collection.splice(index, 1);
  cleanupFieldCollection(spec, section, surface, collectionKey, collection);
  return next as PresentationDraftAST;
}

function mutableActionCollection(document: PresentationDraftAST): {
  next: MutableJSONObject;
  spec: MutableJSONObject;
  actions: MutableJSONValue[];
} {
  const next = mutableDocument(document);
  const spec = ensureObject(next, 'spec');
  const current = spec.actions;
  const actions = Array.isArray(current) ? current : [];
  spec.actions = actions;
  return { next, spec, actions };
}

function mutateActionPatch(
  document: PresentationDraftAST,
  action: string,
  mutate: (patch: MutableJSONObject) => void,
): PresentationDraftAST {
  const { next, spec, actions } = mutableActionCollection(document);
  const existingIndex = findPatchIndex(actions, 'action', action);
  const patch =
    existingIndex >= 0 && isObject(actions[existingIndex])
      ? (actions[existingIndex] as MutableJSONObject)
      : ({ action } as MutableJSONObject);
  mutate(patch);
  if (Object.keys(patch).length === 1 && patch.action === action) {
    if (existingIndex >= 0) actions.splice(existingIndex, 1);
  } else if (existingIndex < 0) {
    actions.push(patch);
  }
  if (actions.length === 0) delete spec.actions;
  return next as PresentationDraftAST;
}

export function presentationActionOverride(
  document: PresentationDraftAST,
  action: string,
): Readonly<Record<string, unknown>> | undefined {
  const actions = presentationDraftSpec(document).actions;
  if (!Array.isArray(actions)) return undefined;
  const patch = actions.find((entry) => isObject(entry) && entry.action === action);
  return isObject(patch) ? patch : undefined;
}

export function setPresentationActionOverride(
  document: PresentationDraftAST,
  action: string,
  property: 'hidden' | 'order' | 'placement',
  value: boolean | number | string | undefined,
): PresentationDraftAST {
  return mutateActionPatch(document, action, (patch) => {
    if (value === undefined) delete patch[property];
    else patch[property] = value;
  });
}

export function setPresentationActionLocalizedOverride(
  document: PresentationDraftAST,
  action: string,
  property: 'confirm' | 'label',
  locale: PresentationDraftLocale,
  value: string | undefined,
): PresentationDraftAST {
  return mutateActionPatch(document, action, (patch) => {
    const localized = ensureObject(patch, property);
    const normalized = value?.trim();
    if (normalized) localized[locale] = normalized;
    else delete localized[locale];
    if (isEmptyObject(localized)) delete patch[property];
  });
}

export function setPresentationActionCondition(
  document: PresentationDraftAST,
  action: string,
  condition: PresentationCondition | undefined,
): PresentationDraftAST {
  return mutateActionPatch(document, action, (patch) => {
    if (condition === undefined) delete patch.visibleWhen;
    else {
      patch.visibleWhen = cloneForWrite(condition as unknown as PresentationJSONValue);
    }
  });
}

export function resetPresentationActionOverrides(
  document: PresentationDraftAST,
  action: string,
): PresentationDraftAST {
  const { next, spec, actions } = mutableActionCollection(document);
  const index = findPatchIndex(actions, 'action', action);
  if (index >= 0) actions.splice(index, 1);
  if (actions.length === 0) delete spec.actions;
  return next as PresentationDraftAST;
}
