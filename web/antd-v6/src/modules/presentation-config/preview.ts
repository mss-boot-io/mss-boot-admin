import {
  type PresentationCapability,
  PresentationContractError,
  type PresentationJSONObject,
} from './contract';

type PreviewLocale = 'en-US' | 'zh-CN';

interface PreviewField {
  field: string;
  label: string;
}

interface PreviewAction {
  action: string;
  label: string;
}

export interface PresentationPreview {
  pageKey: string;
  status: 'ready';
  title: string;
  dataSource: string;
  list: {
    columns: PreviewField[];
    density: 'compact' | 'large' | 'middle';
    pageSize: number;
  };
  actions: PreviewAction[];
}

const record = (value: unknown): Record<string, unknown> | undefined =>
  typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;

function localized(base: unknown, patch: unknown, locale: PreviewLocale, fallback: string) {
  const value = { ...record(base), ...record(patch) };
  const alternate = locale === 'zh-CN' ? 'en-US' : 'zh-CN';
  for (const key of [locale, alternate]) {
    const text = value[key];
    if (typeof text === 'string' && text.trim()) return text.trim();
  }
  return fallback;
}

function patches(value: unknown, key: 'action' | 'field') {
  const result = new Map<string, Record<string, unknown>>();
  if (!Array.isArray(value)) return result;
  for (const candidate of value) {
    const current = record(candidate);
    if (current && typeof current[key] === 'string') result.set(current[key], current);
  }
  return result;
}

export function buildPresentationPreview(
  capability: PresentationCapability,
  document: PresentationJSONObject,
  locale: PreviewLocale,
): PresentationPreview {
  const metadata = record(document.metadata);
  const scope = record(metadata?.scope)?.kind;
  if (
    metadata?.pageKey !== capability.pageKey ||
    metadata.definitionHash !== capability.definitionHash ||
    (scope !== 'application' && scope !== 'role' && scope !== 'user')
  ) {
    throw new PresentationContractError('Invalid presentation identity');
  }
  const spec = record(document.spec);
  if (!spec) throw new PresentationContractError('Invalid presentation spec');
  const defaults = capability.definition.defaultPresentation;
  const listPatch = record(spec.list);
  const dataSource = typeof spec.dataSource === 'string' ? spec.dataSource : defaults.dataSource;
  if (!capability.dataSources.includes(dataSource)) {
    throw new PresentationContractError('Invalid presentation data source');
  }
  const density = listPatch?.density ?? defaults.list.density;
  const pageSize = listPatch?.pageSize ?? defaults.list.pageSize;
  if (
    (density !== 'compact' && density !== 'middle' && density !== 'large') ||
    !Number.isSafeInteger(pageSize) ||
    Number(pageSize) < 1
  ) {
    throw new PresentationContractError('Invalid presentation list');
  }

  const fieldPatches = patches(listPatch?.columns, 'field');
  const fieldDefinitions = new Map(capability.definition.fields.map((field) => [field.id, field]));
  const columns = defaults.list.columns
    .map((field) => {
      const patch = fieldPatches.get(field.field);
      return {
        field: field.field,
        hidden: typeof patch?.hidden === 'boolean' ? patch.hidden : field.hidden,
        order: typeof patch?.order === 'number' ? patch.order : field.order,
        label: localized(
          fieldDefinitions.get(field.field)?.label,
          { ...record(field.label), ...record(patch?.label) },
          locale,
          field.field,
        ),
      };
    })
    .filter((field) => !field.hidden)
    .sort((left, right) => left.order - right.order || left.field.localeCompare(right.field));

  const actionPatches = patches(spec.actions, 'action');
  const actions = defaults.actions
    .map((action) => {
      const patch = actionPatches.get(action.action);
      return {
        action: action.action,
        hidden: typeof patch?.hidden === 'boolean' ? patch.hidden : action.hidden,
        order: typeof patch?.order === 'number' ? patch.order : action.order,
        label: localized(action.label, patch?.label, locale, action.action),
      };
    })
    .filter((action) => !action.hidden)
    .sort((left, right) => left.order - right.order || left.action.localeCompare(right.action));

  return {
    pageKey: capability.pageKey,
    status: 'ready',
    title: localized(defaults.title, spec.title, locale, capability.pageKey),
    dataSource,
    list: { columns, density, pageSize: Number(pageSize) },
    actions,
  };
}
