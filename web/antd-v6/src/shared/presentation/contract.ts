export const ADMIN_PRESENTATION_API_VERSION = 'mss.io/v1alpha1' as const;
export const ADMIN_PRESENTATION_KIND = 'AdminPagePresentation' as const;

export type AdminLocale = 'zh-CN' | 'en-US';
export type PresentationSurface = 'list' | 'search' | 'form' | 'detail';
export type PresentationLayer = 'application' | 'role' | 'user';
export type PresentationDensity = 'compact' | 'middle' | 'large';
export type PresentationActionPlacement = 'toolbar' | 'row' | 'form' | 'detail';
export type PresentationSortDirection = 'asc' | 'desc';
export type PresentationValueType =
  | 'string'
  | 'integer'
  | 'number'
  | 'boolean'
  | 'enum'
  | 'date'
  | 'date-time'
  | 'json';
export type PresentationFieldFormat = 'plain' | 'email' | 'identifier' | 'date' | 'date-time';

export interface LocalizedText {
  'zh-CN'?: string;
  'en-US'?: string;
}

export type PresentationScalar = string | number | boolean | null;
export type PresentationPredicateOperator =
  | 'eq'
  | 'neq'
  | 'in'
  | 'not-in'
  | 'exists'
  | 'not-exists'
  | 'gt'
  | 'gte'
  | 'lt'
  | 'lte';

export type PresentationCondition =
  | {
      field: string;
      operator: PresentationPredicateOperator;
      value?: PresentationScalar | readonly PresentationScalar[];
    }
  | { all: readonly PresentationCondition[] }
  | { any: readonly PresentationCondition[] }
  | { not: PresentationCondition };

export interface PageCapabilityField {
  id: string;
  label: LocalizedText;
  valueType: PresentationValueType;
  // Empty string is a historical v1 Go-wire zero value. Version 2 requires a
  // concrete catalog format and validation rejects the empty value there.
  format?: PresentationFieldFormat | '';
  required: boolean;
  nullable?: boolean;
  readOnly?: boolean;
  searchable?: boolean;
  sortable: boolean;
  filterable: boolean;
  surfaces: readonly PresentationSurface[];
  components: readonly string[];
  validation?: {
    minLength?: number;
    maxLength?: number;
    minimum?: string;
    maximum?: string;
    pattern?: string;
    precision?: number;
    scale?: number;
  };
  surfaceComponents?: readonly {
    surface: PresentationSurface;
    components: readonly string[];
  }[];
  enumValues?: readonly {
    value: string;
    label: LocalizedText;
    color?: string;
  }[];
}

export interface PageCapabilityComponent {
  id: string;
}

export interface PageCapabilityDataSource {
  id: string;
  requiredPermissions: readonly string[];
  pageSizeOptions?: readonly number[];
  maxPageSize?: number;
  maxSortFields?: number;
}

export interface PageCapabilityAction {
  id: string;
  requiredPermissions: readonly string[];
  placements: readonly PresentationActionPlacement[];
  destructive?: boolean;
}

export interface PageFieldPresentation {
  field: string;
  label?: LocalizedText;
  component: string;
  order: number;
  hidden: boolean;
  width?: number;
  span?: number;
  placeholder?: LocalizedText;
  help?: LocalizedText;
  visibleWhen?: PresentationCondition;
}

export interface PageFieldPresentationPatch {
  field: string;
  label?: LocalizedText;
  component?: string;
  order?: number;
  hidden?: boolean;
  width?: number;
  span?: number;
  placeholder?: LocalizedText;
  help?: LocalizedText;
  visibleWhen?: PresentationCondition;
}

export interface PageActionPresentation {
  action: string;
  label?: LocalizedText;
  placement: PresentationActionPlacement;
  order: number;
  hidden: boolean;
  confirm?: LocalizedText;
  visibleWhen?: PresentationCondition;
}

export interface PageActionPresentationPatch {
  action: string;
  label?: LocalizedText;
  placement?: PresentationActionPlacement;
  order?: number;
  hidden?: boolean;
  confirm?: LocalizedText;
  visibleWhen?: PresentationCondition;
}

export interface PageSort {
  field: string;
  direction: PresentationSortDirection;
}

export interface ResolvedPagePresentation {
  title: LocalizedText;
  dataSource: string;
  list: {
    columns: readonly PageFieldPresentation[];
    density: PresentationDensity;
    pageSize: number;
    defaultSort: readonly PageSort[];
  };
  search: {
    fields: readonly PageFieldPresentation[];
    collapsedByDefault: boolean;
  };
  form: {
    fields: readonly PageFieldPresentation[];
    columns: number;
  };
  detail: {
    fields: readonly PageFieldPresentation[];
    columns: number;
  };
  actions: readonly PageActionPresentation[];
}

export interface PageCapabilityDefinition {
  pageKey: string;
  definitionVersion: string;
  definitionHash: string;
  components: readonly PageCapabilityComponent[];
  fields: readonly PageCapabilityField[];
  dataSources: readonly PageCapabilityDataSource[];
  actions: readonly PageCapabilityAction[];
  defaultPresentation: ResolvedPagePresentation;
}

export interface PagePresentationSpec {
  title?: LocalizedText;
  dataSource?: string;
  list?: {
    columns?: readonly PageFieldPresentationPatch[];
    density?: PresentationDensity;
    pageSize?: number;
    defaultSort?: readonly PageSort[];
  };
  search?: {
    fields?: readonly PageFieldPresentationPatch[];
    collapsedByDefault?: boolean;
  };
  form?: {
    fields?: readonly PageFieldPresentationPatch[];
    columns?: number;
  };
  detail?: {
    fields?: readonly PageFieldPresentationPatch[];
    columns?: number;
  };
  actions?: readonly PageActionPresentationPatch[];
}

export type PagePresentationScope =
  | { kind: 'application' }
  | { kind: 'role'; subject: string }
  | { kind: 'user'; subject: string };

export interface AdminPagePresentationProfile {
  apiVersion: typeof ADMIN_PRESENTATION_API_VERSION;
  kind: typeof ADMIN_PRESENTATION_KIND;
  metadata: {
    name: string;
    pageKey: string;
    description?: string;
    definitionHash: string;
    scope: PagePresentationScope;
  };
  spec: PagePresentationSpec;
}

export interface PresentationLayers {
  application?: AdminPagePresentationProfile;
  role?: AdminPagePresentationProfile;
  user?: AdminPagePresentationProfile;
}

export interface PresentationIssue {
  code: string;
  path: string;
  message: string;
}

export interface RejectedPresentationLayer {
  layer: PresentationLayer;
  profileName: string;
  issues: readonly PresentationIssue[];
}

export interface PresentationResolution {
  presentation: ResolvedPagePresentation;
  authorized: boolean;
  appliedLayers: readonly PresentationLayer[];
  rejectedLayers: readonly RejectedPresentationLayer[];
  removedActionIds: readonly string[];
}

export interface PageRenderField {
  field: string;
  label: string;
  component: string;
  order: number;
  width?: number;
  span?: number;
  placeholder?: string;
  help?: string;
  visibleWhen?: PresentationCondition;
}

export interface PageRenderAction {
  action: string;
  label: string;
  placement: PresentationActionPlacement;
  order: number;
  confirm?: string;
  visibleWhen?: PresentationCondition;
}

export interface PageRenderModel {
  pageKey: string;
  status: 'ready' | 'permission-denied';
  title: string;
  dataSource?: string;
  list: {
    columns: readonly PageRenderField[];
    density: PresentationDensity;
    pageSize: number;
    defaultSort: readonly PageSort[];
  };
  search: {
    fields: readonly PageRenderField[];
    collapsedByDefault: boolean;
  };
  form: {
    fields: readonly PageRenderField[];
    columns: number;
  };
  detail: {
    fields: readonly PageRenderField[];
    columns: number;
  };
  actions: readonly PageRenderAction[];
}

const layerOrder: readonly PresentationLayer[] = ['application', 'role', 'user'];
const identifierPattern = /^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$/;
const fieldIdentifierPattern = /^[a-z][A-Za-z0-9]*$/;
const enumValuePattern = /^[a-z][a-z0-9_-]*$/;
const definitionHashPattern = /^sha256:[0-9a-f]{64}$/;
const protectedPageNamespaces = new Set([
  'account',
  'app-config',
  'application-config',
  'auth',
  'authentication',
  'authorization',
  'config',
  'configuration',
  'login',
  'presentation',
  'presentation-config',
  'recovery',
  'release',
  'system',
]);
const presentationValueTypes = new Set<PresentationValueType>([
  'string',
  'integer',
  'number',
  'boolean',
  'enum',
  'date',
  'date-time',
  'json',
]);
const presentationFieldFormats = new Set<PresentationFieldFormat>([
  'plain',
  'email',
  'identifier',
  'date',
  'date-time',
]);
const predicateOperators = new Set<PresentationPredicateOperator>([
  'eq',
  'neq',
  'in',
  'not-in',
  'exists',
  'not-exists',
  'gt',
  'gte',
  'lt',
  'lte',
]);
const forbiddenProfileKeys = new Set([
  'permission',
  'permissions',
  'route',
  'url',
  'method',
  'headers',
  'html',
  'script',
  'expression',
  'sql',
  'import',
  'componentImport',
  'plugin',
  'template',
]);

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

export function canonicalizeCapabilityContract(value: unknown): string {
  if (value === null || typeof value === 'string' || typeof value === 'boolean') {
    return JSON.stringify(value);
  }
  if (typeof value === 'number') {
    if (!Number.isSafeInteger(value)) {
      throw new Error('Capability contract numbers must be safe integers');
    }
    return JSON.stringify(value);
  }
  if (Array.isArray(value)) {
    return `[${value.map(canonicalizeCapabilityContract).join(',')}]`;
  }
  if (isRecord(value)) {
    const properties = Object.entries(value)
      .filter(([, nested]) => nested !== undefined)
      .sort(([left], [right]) => (left < right ? -1 : left > right ? 1 : 0))
      .map(([key, nested]) => `${JSON.stringify(key)}:${canonicalizeCapabilityContract(nested)}`);
    return `{${properties.join(',')}}`;
  }
  throw new Error(`Unsupported capability contract value: ${typeof value}`);
}

function addIssue(issues: PresentationIssue[], code: string, path: string, message: string) {
  issues.push({ code, path, message });
}

function duplicateIDs(values: readonly string[]): readonly string[] {
  const seen = new Set<string>();
  const duplicates = new Set<string>();
  for (const value of values) {
    if (seen.has(value)) duplicates.add(value);
    seen.add(value);
  }
  return [...duplicates].sort();
}

function cloneLocalized(value: LocalizedText | undefined): LocalizedText | undefined {
  return value ? { ...value } : undefined;
}

function mergeLocalized(
  current: LocalizedText | undefined,
  patch: LocalizedText | undefined,
): LocalizedText | undefined {
  if (!patch) return cloneLocalized(current);
  return { ...current, ...patch };
}

function cloneCondition(
  condition: PresentationCondition | undefined,
): PresentationCondition | undefined {
  if (!condition) return undefined;
  if ('all' in condition) {
    return { all: condition.all.map((item) => cloneCondition(item) as PresentationCondition) };
  }
  if ('any' in condition) {
    return { any: condition.any.map((item) => cloneCondition(item) as PresentationCondition) };
  }
  if ('not' in condition) {
    return { not: cloneCondition(condition.not) as PresentationCondition };
  }
  return {
    field: condition.field,
    operator: condition.operator,
    value: Array.isArray(condition.value) ? [...condition.value] : condition.value,
  };
}

function cloneField(field: PageFieldPresentation): PageFieldPresentation {
  return {
    ...field,
    label: cloneLocalized(field.label),
    placeholder: cloneLocalized(field.placeholder),
    help: cloneLocalized(field.help),
    visibleWhen: cloneCondition(field.visibleWhen),
  };
}

function cloneAction(action: PageActionPresentation): PageActionPresentation {
  return {
    ...action,
    label: cloneLocalized(action.label),
    confirm: cloneLocalized(action.confirm),
    visibleWhen: cloneCondition(action.visibleWhen),
  };
}

function clonePresentation(value: ResolvedPagePresentation): ResolvedPagePresentation {
  return {
    title: { ...value.title },
    dataSource: value.dataSource,
    list: {
      ...value.list,
      columns: value.list.columns.map(cloneField),
      defaultSort: value.list.defaultSort.map((sort) => ({ ...sort })),
    },
    search: { ...value.search, fields: value.search.fields.map(cloneField) },
    form: { ...value.form, fields: value.form.fields.map(cloneField) },
    detail: { ...value.detail, fields: value.detail.fields.map(cloneField) },
    actions: value.actions.map(cloneAction),
  };
}

function validateLocalizedText(
  value: LocalizedText | undefined,
  path: string,
  issues: PresentationIssue[],
) {
  if (!value) return;
  if (!value['zh-CN'] && !value['en-US']) {
    addIssue(issues, 'empty-localized-text', path, 'Localized text must contain zh-CN or en-US');
  }
}

function validateCondition(
  condition: PresentationCondition | undefined,
  fields: ReadonlySet<string>,
  path: string,
  issues: PresentationIssue[],
  depth = 0,
) {
  if (!condition) return;
  if (depth > 8) {
    addIssue(issues, 'condition-depth', path, 'Condition depth exceeds the limit of eight');
    return;
  }
  if ('all' in condition) {
    const children = condition.all;
    if (children.length === 0 || children.length > 20) {
      addIssue(issues, 'condition-size', `${path}.all`, 'Condition group must have 1 to 20 items');
    }
    children.forEach((child, index) => {
      validateCondition(child, fields, `${path}.all[${index}]`, issues, depth + 1);
    });
    return;
  }
  if ('any' in condition) {
    const children = condition.any;
    if (children.length === 0 || children.length > 20) {
      addIssue(issues, 'condition-size', `${path}.any`, 'Condition group must have 1 to 20 items');
    }
    children.forEach((child, index) => {
      validateCondition(child, fields, `${path}.any[${index}]`, issues, depth + 1);
    });
    return;
  }
  if ('not' in condition) {
    validateCondition(condition.not, fields, `${path}.not`, issues, depth + 1);
    return;
  }
  if (!fields.has(condition.field)) {
    addIssue(
      issues,
      'unknown-condition-field',
      `${path}.field`,
      `Unknown condition field ${condition.field}`,
    );
  }
  if (!predicateOperators.has(condition.operator)) {
    addIssue(
      issues,
      'unknown-condition-operator',
      `${path}.operator`,
      `Unknown condition operator ${condition.operator}`,
    );
  }
  const presenceOperator = condition.operator === 'exists' || condition.operator === 'not-exists';
  if (presenceOperator && condition.value !== undefined) {
    addIssue(
      issues,
      'unexpected-condition-value',
      `${path}.value`,
      'Presence operators take no value',
    );
  }
  if (!presenceOperator && condition.value === undefined) {
    addIssue(
      issues,
      'missing-condition-value',
      `${path}.value`,
      'Comparison operator needs a value',
    );
  }
  if (Array.isArray(condition.value) && condition.value.length > 50) {
    addIssue(issues, 'condition-value-size', `${path}.value`, 'Condition value has over 50 items');
  }
}

function inspectForbiddenKeys(value: unknown, path: string, issues: PresentationIssue[]) {
  if (Array.isArray(value)) {
    value.forEach((item, index) => {
      inspectForbiddenKeys(item, `${path}[${index}]`, issues);
    });
    return;
  }
  if (!isRecord(value)) return;
  for (const [key, nested] of Object.entries(value)) {
    if (forbiddenProfileKeys.has(key)) {
      addIssue(
        issues,
        'forbidden-profile-key',
        `${path}.${key}`,
        `Presentation profiles cannot contain ${key}`,
      );
    }
    inspectForbiddenKeys(nested, `${path}.${key}`, issues);
  }
}

function validateFieldCollection(
  capability: PageCapabilityDefinition,
  surface: PresentationSurface,
  fields: readonly PageFieldPresentationPatch[],
  path: string,
  issues: PresentationIssue[],
) {
  const fieldDefinitions = new Map(capability.fields.map((field) => [field.id, field]));
  const componentIDs = new Set(capability.components.map((component) => component.id));
  for (const duplicate of duplicateIDs(fields.map((field) => field.field))) {
    addIssue(issues, 'duplicate-field', path, `Field ${duplicate} is configured more than once`);
  }
  fields.forEach((field, index) => {
    const fieldPath = `${path}[${index}]`;
    const definition = fieldDefinitions.get(field.field);
    if (!definition) {
      addIssue(issues, 'unknown-field', `${fieldPath}.field`, `Unknown field ${field.field}`);
      return;
    }
    if (!definition.surfaces.includes(surface)) {
      addIssue(
        issues,
        'unsupported-field-surface',
        `${fieldPath}.field`,
        `Field ${field.field} is not registered for ${surface}`,
      );
    }
    if (field.component) {
      if (!componentIDs.has(field.component)) {
        addIssue(
          issues,
          'unknown-component',
          `${fieldPath}.component`,
          `Unknown component ${field.component}`,
        );
      } else if (
        !fieldSupportsComponentOnSurface(capability, definition, surface, field.component)
      ) {
        addIssue(
          issues,
          'unsupported-field-component',
          `${fieldPath}.component`,
          `Component ${field.component} is not allowed for ${field.field}`,
        );
      }
    }
    if (surface === 'form' && definition.required && field.hidden === true) {
      addIssue(
        issues,
        'required-form-field-hidden',
        `${fieldPath}.hidden`,
        `Required form field ${field.field} cannot be hidden`,
      );
    }
    if (surface === 'form' && definition.required && field.visibleWhen !== undefined) {
      addIssue(
        issues,
        'required-form-field-conditional',
        `${fieldPath}.visibleWhen`,
        `Required form field ${field.field} cannot be conditionally hidden`,
      );
    }
    if (field.order !== undefined && (!Number.isInteger(field.order) || field.order < 0)) {
      addIssue(
        issues,
        'invalid-field-order',
        `${fieldPath}.order`,
        'Field order must be non-negative',
      );
    }
    if (field.width !== undefined && (field.width < 60 || field.width > 1200)) {
      addIssue(
        issues,
        'invalid-field-width',
        `${fieldPath}.width`,
        'Field width must be 60 to 1200',
      );
    }
    if (field.span !== undefined && (field.span < 1 || field.span > 24)) {
      addIssue(issues, 'invalid-field-span', `${fieldPath}.span`, 'Field span must be 1 to 24');
    }
    validateLocalizedText(field.label, `${fieldPath}.label`, issues);
    validateLocalizedText(field.placeholder, `${fieldPath}.placeholder`, issues);
    validateLocalizedText(field.help, `${fieldPath}.help`, issues);
    validateCondition(
      field.visibleWhen,
      new Set(fieldDefinitions.keys()),
      `${fieldPath}.visibleWhen`,
      issues,
    );
  });
}

function fieldSupportsComponentOnSurface(
  capability: PageCapabilityDefinition,
  field: PageCapabilityField,
  surface: PresentationSurface,
  component: string,
): boolean {
  if (capability.definitionVersion !== '2') return field.components.includes(component);
  return (
    field.surfaceComponents
      ?.find((mapping) => mapping.surface === surface)
      ?.components.includes(component) === true
  );
}

function surfaceFields(
  presentation: ResolvedPagePresentation,
  surface: PresentationSurface,
): readonly PageFieldPresentation[] {
  if (surface === 'list') return presentation.list.columns;
  return presentation[surface].fields;
}

function validateDataSourceLimits(
  dataSource: PageCapabilityDataSource,
  index: number,
  issues: PresentationIssue[],
) {
  const path = `dataSources[${index}]`;
  if (
    dataSource.maxPageSize === undefined ||
    !Number.isSafeInteger(dataSource.maxPageSize) ||
    dataSource.maxPageSize < 1 ||
    dataSource.maxPageSize > 200
  ) {
    addIssue(
      issues,
      'invalid-max-page-size',
      `${path}.maxPageSize`,
      'Maximum page size must be 1 to 200',
    );
  }
  if (!dataSource.pageSizeOptions?.length) {
    addIssue(
      issues,
      'missing-page-size-options',
      `${path}.pageSizeOptions`,
      'Version 2 data source needs page size options',
    );
  } else {
    const seen = new Set<number>();
    dataSource.pageSizeOptions.forEach((option, optionIndex) => {
      const optionPath = `${path}.pageSizeOptions[${optionIndex}]`;
      if (
        !Number.isSafeInteger(option) ||
        option < 1 ||
        (dataSource.maxPageSize !== undefined && option > dataSource.maxPageSize)
      ) {
        addIssue(
          issues,
          'invalid-page-size-option',
          optionPath,
          'Page size option must be a safe integer within the data source maximum',
        );
      }
      if (seen.has(option)) {
        addIssue(
          issues,
          'duplicate-page-size-option',
          optionPath,
          'Page size option is duplicated',
        );
      }
      const previousOption = dataSource.pageSizeOptions?.[optionIndex - 1];
      if (previousOption !== undefined && option <= previousOption) {
        addIssue(
          issues,
          'unsorted-page-size-options',
          optionPath,
          'Page size options must be strictly increasing',
        );
      }
      seen.add(option);
    });
  }
  if (
    dataSource.maxSortFields === undefined ||
    !Number.isSafeInteger(dataSource.maxSortFields) ||
    dataSource.maxSortFields < 0 ||
    dataSource.maxSortFields > 3
  ) {
    addIssue(
      issues,
      'invalid-max-sort-fields',
      `${path}.maxSortFields`,
      'Maximum sort fields must be 0 to 3',
    );
  }
}

function validateCapabilityNumericFacts(
  field: PageCapabilityField,
  fieldPath: string,
  issues: PresentationIssue[],
) {
  const validation = field.validation;
  if (!validation) return;
  const decimalPattern = /^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:e[+-]?\d+)?$/i;
  const parseBound = (value: string | undefined, path: string): number | undefined => {
    if (value === undefined) return undefined;
    const parsed = Number(value);
    if (!decimalPattern.test(value) || !Number.isFinite(parsed)) {
      addIssue(
        issues,
        'invalid-field-number-bound',
        path,
        'Numeric bound must be a finite decimal string',
      );
      return undefined;
    }
    return parsed;
  };
  const minimum = parseBound(validation.minimum, `${fieldPath}.validation.minimum`);
  const maximum = parseBound(validation.maximum, `${fieldPath}.validation.maximum`);
  if (minimum !== undefined && maximum !== undefined && minimum > maximum) {
    addIssue(
      issues,
      'invalid-field-number-range',
      `${fieldPath}.validation`,
      'Minimum cannot exceed maximum',
    );
  }
  if (
    validation.precision !== undefined &&
    (!Number.isSafeInteger(validation.precision) ||
      validation.precision < 1 ||
      validation.precision > 38)
  ) {
    addIssue(
      issues,
      'invalid-field-precision',
      `${fieldPath}.validation.precision`,
      'Precision must be a safe integer from 1 to 38',
    );
  }
  if (
    validation.scale !== undefined &&
    (!Number.isSafeInteger(validation.scale) || validation.scale < 0 || validation.scale > 38)
  ) {
    addIssue(
      issues,
      'invalid-field-scale',
      `${fieldPath}.validation.scale`,
      'Scale must be a safe integer from 0 to 38',
    );
  }
  if (
    validation.scale !== undefined &&
    validation.precision !== undefined &&
    validation.scale > validation.precision
  ) {
    addIssue(
      issues,
      'invalid-field-scale',
      `${fieldPath}.validation.scale`,
      'Scale cannot exceed precision',
    );
  }
}

function validateSurfaceComponents(
  field: PageCapabilityField,
  fieldPath: string,
  componentIDs: ReadonlySet<string>,
  issues: PresentationIssue[],
) {
  const mappings = field.surfaceComponents;
  if (!mappings?.length) {
    addIssue(
      issues,
      'missing-surface-components',
      `${fieldPath}.surfaceComponents`,
      'Version 2 field needs component choices for every surface',
    );
    return;
  }
  const declaredSurfaces = new Set(field.surfaces);
  const declaredComponents = new Set(field.components);
  const seenSurfaces = new Set<PresentationSurface>();
  const usedComponents = new Set<string>();
  mappings.forEach((mapping, mappingIndex) => {
    const mappingPath = `${fieldPath}.surfaceComponents[${mappingIndex}]`;
    if (!declaredSurfaces.has(mapping.surface)) {
      addIssue(
        issues,
        'unexpected-surface-components',
        `${mappingPath}.surface`,
        'Surface components reference an undeclared field surface',
      );
    }
    if (seenSurfaces.has(mapping.surface)) {
      addIssue(
        issues,
        'duplicate-surface-components',
        `${mappingPath}.surface`,
        'Surface component mapping is duplicated',
      );
    }
    seenSurfaces.add(mapping.surface);
    if (mapping.components.length === 0) {
      addIssue(
        issues,
        'missing-surface-component',
        `${mappingPath}.components`,
        'Surface needs at least one component',
      );
    }
    const seenComponents = new Set<string>();
    mapping.components.forEach((component, componentIndex) => {
      const componentPath = `${mappingPath}.components[${componentIndex}]`;
      if (!componentIDs.has(component)) {
        addIssue(
          issues,
          'unknown-field-component',
          componentPath,
          'Surface references an unknown component',
        );
      }
      if (!declaredComponents.has(component)) {
        addIssue(
          issues,
          'surface-component-mismatch',
          componentPath,
          'Surface component is missing from the field component inventory',
        );
      }
      if (seenComponents.has(component)) {
        addIssue(
          issues,
          'duplicate-field-component',
          componentPath,
          'Surface component is duplicated',
        );
      }
      seenComponents.add(component);
      usedComponents.add(component);
    });
  });
  for (const surface of declaredSurfaces) {
    if (!seenSurfaces.has(surface)) {
      addIssue(
        issues,
        'missing-surface-components',
        `${fieldPath}.surfaceComponents`,
        `Surface component mapping is missing for ${surface}`,
      );
    }
  }
  for (const component of declaredComponents) {
    if (!usedComponents.has(component)) {
      addIssue(
        issues,
        'surface-component-mismatch',
        `${fieldPath}.surfaceComponents`,
        `Field component is not available on any surface: ${component}`,
      );
    }
  }
}

function validatePageSizeAgainstDataSource(
  pageSize: number,
  dataSource: PageCapabilityDataSource,
  path: string,
  issues: PresentationIssue[],
) {
  if (dataSource.maxPageSize !== undefined && pageSize > dataSource.maxPageSize) {
    addIssue(
      issues,
      'page-size-exceeds-data-source-limit',
      path,
      'Page size exceeds the compiled data source maximum',
    );
  }
  if (dataSource.pageSizeOptions && !dataSource.pageSizeOptions.includes(pageSize)) {
    addIssue(
      issues,
      'unsupported-page-size',
      path,
      'Page size is not allowed by the compiled data source',
    );
  }
}

function validateCompletePresentation(
  capability: PageCapabilityDefinition,
  presentation: ResolvedPagePresentation,
): PresentationIssue[] {
  const issues: PresentationIssue[] = [];
  const dataSources = new Map(
    capability.dataSources.map((dataSource) => [dataSource.id, dataSource]),
  );
  const dataSource = dataSources.get(presentation.dataSource);
  if (!dataSource) {
    addIssue(
      issues,
      'unknown-data-source',
      'defaultPresentation.dataSource',
      `Unknown data source ${presentation.dataSource}`,
    );
  }
  for (const surface of ['list', 'search', 'form', 'detail'] as const) {
    validateFieldCollection(
      capability,
      surface,
      surfaceFields(presentation, surface),
      `defaultPresentation.${surface}`,
      issues,
    );
  }
  for (const field of capability.fields) {
    for (const surface of field.surfaces) {
      if (!surfaceFields(presentation, surface).some((item) => item.field === field.id)) {
        addIssue(
          issues,
          'missing-default-field',
          `defaultPresentation.${surface}`,
          `Field ${field.id} needs a default entry for ${surface}`,
        );
      }
    }
  }
  if (presentation.list.columns.every((field) => field.hidden)) {
    addIssue(
      issues,
      'empty-list-presentation',
      'defaultPresentation.list.columns',
      'At least one list column must remain visible',
    );
  }
  if (
    !Number.isSafeInteger(presentation.list.pageSize) ||
    presentation.list.pageSize < 1 ||
    presentation.list.pageSize > 200
  ) {
    addIssue(
      issues,
      'invalid-page-size',
      'defaultPresentation.list.pageSize',
      'Default page size must be a safe integer from 1 to 200',
    );
  }
  for (const surface of ['form', 'detail'] as const) {
    const columns = presentation[surface].columns;
    if (!Number.isSafeInteger(columns) || columns < 1 || columns > 4) {
      addIssue(
        issues,
        'invalid-layout-columns',
        `defaultPresentation.${surface}.columns`,
        'Layout columns must be a safe integer from 1 to 4',
      );
    }
  }
  if (capability.definitionVersion === '2' && dataSource) {
    validatePageSizeAgainstDataSource(
      presentation.list.pageSize,
      dataSource,
      'defaultPresentation.list.pageSize',
      issues,
    );
    if (
      dataSource.maxSortFields !== undefined &&
      presentation.list.defaultSort.length > dataSource.maxSortFields
    ) {
      addIssue(
        issues,
        'too-many-data-source-sort-fields',
        'defaultPresentation.list.defaultSort',
        'Default sort exceeds the compiled data source limit',
      );
    }
  }
  const fields = new Map(capability.fields.map((field) => [field.id, field]));
  for (const [index, sort] of presentation.list.defaultSort.entries()) {
    const definition = fields.get(sort.field);
    if (!definition) {
      addIssue(
        issues,
        'unknown-sort-field',
        `defaultPresentation.list.defaultSort[${index}].field`,
        `Unknown sort field ${sort.field}`,
      );
    } else if (!definition.sortable) {
      addIssue(
        issues,
        'unsupported-sort-field',
        `defaultPresentation.list.defaultSort[${index}].field`,
        `Field ${sort.field} is not sortable`,
      );
    }
  }
  const actionDefinitions = new Map(capability.actions.map((action) => [action.id, action]));
  for (const duplicate of duplicateIDs(presentation.actions.map((action) => action.action))) {
    addIssue(
      issues,
      'duplicate-action',
      'defaultPresentation.actions',
      `Action ${duplicate} is configured more than once`,
    );
  }
  presentation.actions.forEach((action, index) => {
    const definition = actionDefinitions.get(action.action);
    if (!definition) {
      addIssue(
        issues,
        'unknown-action',
        `defaultPresentation.actions[${index}].action`,
        `Unknown action ${action.action}`,
      );
    } else if (!definition.placements.includes(action.placement)) {
      addIssue(
        issues,
        'unsupported-action-placement',
        `defaultPresentation.actions[${index}].placement`,
        `Action ${action.action} cannot be placed at ${action.placement}`,
      );
    }
  });
  return issues;
}

export function validatePageCapabilityDefinition(
  capability: PageCapabilityDefinition,
): PresentationIssue[] {
  const issues: PresentationIssue[] = [];
  if (
    capability.pageKey.length < 2 ||
    capability.pageKey.length > 120 ||
    !identifierPattern.test(capability.pageKey)
  ) {
    addIssue(issues, 'invalid-page-key', 'pageKey', 'Page key must be a stable identifier');
  }
  if (protectedPageNamespaces.has(capability.pageKey.split('.', 1)[0] ?? '')) {
    addIssue(
      issues,
      'protected-page',
      'pageKey',
      'Protected core page cannot be presentation-configurable',
    );
  }
  if (!capability.definitionVersion.trim()) {
    addIssue(
      issues,
      'missing-definition-version',
      'definitionVersion',
      'Definition version is required',
    );
  } else if (capability.definitionVersion !== '1' && capability.definitionVersion !== '2') {
    addIssue(
      issues,
      'unsupported-definition-version',
      'definitionVersion',
      'Definition version must be 1 or 2',
    );
  }
  if (!definitionHashPattern.test(capability.definitionHash)) {
    addIssue(
      issues,
      'invalid-definition-hash',
      'definitionHash',
      'Definition hash must be sha256 followed by 64 lowercase hexadecimal characters',
    );
  }
  for (const [path, values] of [
    ['components', capability.components.map((item) => item.id)],
    ['fields', capability.fields.map((item) => item.id)],
    ['dataSources', capability.dataSources.map((item) => item.id)],
    ['actions', capability.actions.map((item) => item.id)],
  ] as const) {
    for (const duplicate of duplicateIDs(values)) {
      addIssue(
        issues,
        'duplicate-capability-id',
        path,
        `${duplicate} is registered more than once`,
      );
    }
  }
  const componentIDs = new Set(capability.components.map((component) => component.id));
  capability.fields.forEach((field, index) => {
    const fieldPath = `fields[${index}]`;
    const validFieldID =
      capability.definitionVersion === '2'
        ? fieldIdentifierPattern.test(field.id)
        : identifierPattern.test(field.id) || fieldIdentifierPattern.test(field.id);
    if (!validFieldID) {
      addIssue(
        issues,
        'invalid-field-identifier',
        `${fieldPath}.id`,
        capability.definitionVersion === '2'
          ? 'Field id must be a stable lower-camel identifier'
          : 'Field id must be a stable version 1 identifier',
      );
    }
    validateLocalizedText(field.label, `${fieldPath}.label`, issues);
    if (!presentationValueTypes.has(field.valueType)) {
      addIssue(
        issues,
        'invalid-field-value-type',
        `${fieldPath}.valueType`,
        'Field value type is not supported',
      );
    }
    if (
      (capability.definitionVersion === '2' && !field.format) ||
      (field.format !== undefined &&
        field.format !== '' &&
        !presentationFieldFormats.has(field.format))
    ) {
      addIssue(
        issues,
        'unsupported-field-format',
        `${fieldPath}.format`,
        'Field format is not supported',
      );
    }
    if (
      capability.definitionVersion === '2' &&
      (typeof field.nullable !== 'boolean' ||
        typeof field.readOnly !== 'boolean' ||
        typeof field.searchable !== 'boolean')
    ) {
      addIssue(
        issues,
        'incomplete-field-facts',
        fieldPath,
        'Version 2 field requires explicit nullable, readOnly, and searchable facts',
      );
    }
    if (
      capability.definitionVersion === '2' &&
      (field.validation === undefined ||
        field.validation === null ||
        typeof field.validation !== 'object' ||
        Array.isArray(field.validation))
    ) {
      addIssue(
        issues,
        'missing-field-validation',
        `${fieldPath}.validation`,
        'Version 2 fields require an explicit validation object',
      );
    }
    if (capability.definitionVersion === '2' && !Array.isArray(field.enumValues)) {
      addIssue(
        issues,
        'missing-enum-values',
        `${fieldPath}.enumValues`,
        'Version 2 fields require an explicit enumValues array, including an empty array',
      );
    }
    if (field.required && field.nullable) {
      addIssue(
        issues,
        'conflicting-field-nullability',
        fieldPath,
        'Required and nullable cannot both be true',
      );
    }
    const minLength = field.validation?.minLength;
    const maxLength = field.validation?.maxLength;
    if (minLength !== undefined && (!Number.isSafeInteger(minLength) || minLength < 0)) {
      addIssue(
        issues,
        'invalid-field-min-length',
        `${fieldPath}.validation.minLength`,
        'Minimum length must be a non-negative safe integer',
      );
    }
    if (maxLength !== undefined && (!Number.isSafeInteger(maxLength) || maxLength < 0)) {
      addIssue(
        issues,
        'invalid-field-max-length',
        `${fieldPath}.validation.maxLength`,
        'Maximum length must be a non-negative safe integer',
      );
    }
    if (minLength !== undefined && maxLength !== undefined && minLength > maxLength) {
      addIssue(
        issues,
        'invalid-field-length-range',
        `${fieldPath}.validation`,
        'Minimum length cannot exceed maximum length',
      );
    }
    if (field.validation?.pattern) {
      try {
        if (capability.definitionVersion === '2') {
          if (isPortableCapabilityPattern(field.validation.pattern)) {
            compilePortablePresentationPattern(field.validation.pattern);
          } else {
            new RegExp(field.validation.pattern, 'u');
          }
        } else {
          new RegExp(field.validation.pattern);
        }
      } catch {
        addIssue(
          issues,
          'invalid-field-pattern',
          `${fieldPath}.validation.pattern`,
          'Field pattern is invalid',
        );
      }
      if (
        capability.definitionVersion === '2' &&
        !isPortableCapabilityPattern(field.validation.pattern)
      ) {
        addIssue(
          issues,
          'non-portable-field-pattern',
          `${fieldPath}.validation.pattern`,
          'Field pattern must use the portable Go and ECMAScript subset',
        );
      }
    }
    validateCapabilityNumericFacts(field, fieldPath, issues);
    const enumValues = new Set<string>();
    field.enumValues?.forEach((enumValue, enumIndex) => {
      const enumPath = `${fieldPath}.enumValues[${enumIndex}]`;
      if (!enumValuePattern.test(enumValue.value)) {
        addIssue(
          issues,
          'invalid-enum-value',
          `${enumPath}.value`,
          'Enum value must be a stable lower-case token',
        );
      }
      validateLocalizedText(enumValue.label, `${enumPath}.label`, issues);
      if (capability.definitionVersion === '2' && typeof enumValue.color !== 'string') {
        addIssue(
          issues,
          'missing-enum-color',
          `${enumPath}.color`,
          'Version 2 enum values require an explicit color string',
        );
      }
      if (enumValues.has(enumValue.value)) {
        addIssue(issues, 'duplicate-enum-value', `${enumPath}.value`, 'Enum value is duplicated');
      }
      enumValues.add(enumValue.value);
    });
    if (
      capability.definitionVersion === '2' &&
      field.valueType === 'enum' &&
      !field.enumValues?.length
    ) {
      addIssue(issues, 'missing-enum-values', `${fieldPath}.enumValues`, 'Enum field needs values');
    }
    if (field.valueType !== 'enum' && field.enumValues?.length) {
      addIssue(
        issues,
        'unexpected-enum-values',
        `${fieldPath}.enumValues`,
        'Enum values require enum value type',
      );
    }
    if (field.surfaces.length === 0) {
      addIssue(
        issues,
        'missing-field-surface',
        `${fieldPath}.surfaces`,
        `Field ${field.id} needs at least one surface`,
      );
    }
    if (field.components.length === 0) {
      addIssue(
        issues,
        'missing-field-component',
        `${fieldPath}.components`,
        `Field ${field.id} needs at least one component`,
      );
    }
    for (const component of field.components) {
      if (!componentIDs.has(component)) {
        addIssue(
          issues,
          'unknown-field-component',
          `${fieldPath}.components`,
          `Field ${field.id} references unknown component ${component}`,
        );
      }
    }
    if (capability.definitionVersion === '2') {
      validateSurfaceComponents(field, fieldPath, componentIDs, issues);
    }
  });
  capability.dataSources.forEach((dataSource, index) => {
    if (
      dataSource.requiredPermissions.length === 0 ||
      dataSource.requiredPermissions.some((permission) => !permission.trim())
    ) {
      addIssue(
        issues,
        'missing-data-source-permission',
        `dataSources[${index}].requiredPermissions`,
        `Data source ${dataSource.id} needs non-empty permissions`,
      );
    }
    if (capability.definitionVersion === '2') {
      validateDataSourceLimits(dataSource, index, issues);
    }
  });
  capability.actions.forEach((action, index) => {
    if (capability.definitionVersion === '2' && typeof action.destructive !== 'boolean') {
      addIssue(
        issues,
        'missing-action-destructive',
        `actions[${index}].destructive`,
        'Version 2 actions require an explicit destructive boolean',
      );
    }
    if (
      action.requiredPermissions.length === 0 ||
      action.requiredPermissions.some((permission) => !permission.trim())
    ) {
      addIssue(
        issues,
        'missing-action-permission',
        `actions[${index}].requiredPermissions`,
        `Action ${action.id} needs non-empty permissions`,
      );
    }
  });
  issues.push(...validateCompletePresentation(capability, capability.defaultPresentation));
  return issues;
}

function isPortableCapabilityPattern(value: string): boolean {
  if (value.includes('(?') || value.includes('[[:')) return false;
  for (const current of value) {
    const codePoint = current.codePointAt(0) ?? 0;
    if (codePoint > 0xffff || codePoint === 0x2028 || codePoint === 0x2029) return false;
  }
  let inClass = false;
  let classHasContent = false;
  let classAtStart = false;
  for (let index = 0; index < value.length; index += 1) {
    const current = value[index];
    if (current === '\\') {
      index += 1;
      if (index >= value.length) return false;
      const escaped = value[index];
      if (escaped === undefined) return false;
      if (escaped === 'x') {
        const hex = value.slice(index + 1, index + 3);
        if (!/^[0-9A-Fa-f]{2}$/.test(hex)) return false;
        index += 2;
      } else {
        const allowed = inClass ? 'dDfnrtvwW$()*+-./?[\\]^{|}' : 'bBdDfnrtvwW$()*+./?[\\]^{|}';
        if (!allowed.includes(escaped)) return false;
      }
      if (inClass) {
        classHasContent = true;
        classAtStart = false;
      }
      continue;
    }
    if (current === '[') {
      if (inClass) {
        classHasContent = true;
        classAtStart = false;
      } else {
        inClass = true;
        classHasContent = false;
        classAtStart = true;
      }
    } else if (current === ']') {
      if (!inClass || !classHasContent) return false;
      inClass = false;
      classHasContent = false;
      classAtStart = false;
    } else if (current === '^') {
      if (inClass) {
        if (classAtStart) classAtStart = false;
        else classHasContent = true;
      }
    } else if (current === '{' || current === '}') {
      if (!inClass) return false;
      classHasContent = true;
      classAtStart = false;
    } else if (current === '.') {
      if (!inClass) return false;
      classHasContent = true;
      classAtStart = false;
    } else if (inClass) {
      classHasContent = true;
      classAtStart = false;
    }
  }
  return !inClass;
}

export function compilePortablePresentationPattern(pattern: string): RegExp {
  if (!isPortableCapabilityPattern(pattern)) {
    throw new Error('Pattern is outside the portable Go and ECMAScript subset');
  }
  return new RegExp(pattern, 'u');
}

export function validatePagePresentationProfile(
  capability: PageCapabilityDefinition,
  profile: AdminPagePresentationProfile,
): PresentationIssue[] {
  const issues: PresentationIssue[] = [];
  inspectForbiddenKeys(profile, '$', issues);
  if (profile.apiVersion !== ADMIN_PRESENTATION_API_VERSION) {
    addIssue(
      issues,
      'unsupported-api-version',
      'apiVersion',
      'Unsupported presentation API version',
    );
  }
  if (profile.kind !== ADMIN_PRESENTATION_KIND) {
    addIssue(issues, 'unsupported-kind', 'kind', 'Unsupported presentation document kind');
  }
  if (profile.metadata.pageKey !== capability.pageKey) {
    addIssue(
      issues,
      'page-key-mismatch',
      'metadata.pageKey',
      `Expected page key ${capability.pageKey}`,
    );
  }
  if (profile.metadata.definitionHash !== capability.definitionHash) {
    addIssue(
      issues,
      'definition-hash-mismatch',
      'metadata.definitionHash',
      'Presentation profile was created for another capability definition',
    );
  }
  if (profile.metadata.scope.kind !== 'application' && !profile.metadata.scope.subject.trim()) {
    addIssue(
      issues,
      'missing-scope-subject',
      'metadata.scope.subject',
      'Scope subject is required',
    );
  }
  const dataSources = new Map(
    capability.dataSources.map((dataSource) => [dataSource.id, dataSource]),
  );
  if (profile.spec.dataSource && !dataSources.has(profile.spec.dataSource)) {
    addIssue(
      issues,
      'unknown-data-source',
      'spec.dataSource',
      `Unknown data source ${profile.spec.dataSource}`,
    );
  }
  const effectiveDataSource = dataSources.get(
    profile.spec.dataSource ?? capability.defaultPresentation.dataSource,
  );
  validateLocalizedText(profile.spec.title, 'spec.title', issues);
  if (profile.spec.list?.columns) {
    validateFieldCollection(
      capability,
      'list',
      profile.spec.list.columns,
      'spec.list.columns',
      issues,
    );
  }
  if (profile.spec.search?.fields) {
    validateFieldCollection(
      capability,
      'search',
      profile.spec.search.fields,
      'spec.search.fields',
      issues,
    );
  }
  if (profile.spec.form?.fields) {
    validateFieldCollection(
      capability,
      'form',
      profile.spec.form.fields,
      'spec.form.fields',
      issues,
    );
  }
  if (profile.spec.detail?.fields) {
    validateFieldCollection(
      capability,
      'detail',
      profile.spec.detail.fields,
      'spec.detail.fields',
      issues,
    );
  }
  const fieldDefinitions = new Map(capability.fields.map((field) => [field.id, field]));
  profile.spec.list?.defaultSort?.forEach((sort, index) => {
    const definition = fieldDefinitions.get(sort.field);
    if (!definition) {
      addIssue(
        issues,
        'unknown-sort-field',
        `spec.list.defaultSort[${index}].field`,
        `Unknown sort field ${sort.field}`,
      );
    } else if (!definition.sortable) {
      addIssue(
        issues,
        'unsupported-sort-field',
        `spec.list.defaultSort[${index}].field`,
        `Field ${sort.field} is not sortable`,
      );
    }
  });
  if (capability.definitionVersion === '2' && effectiveDataSource) {
    if (profile.spec.list?.pageSize !== undefined) {
      validatePageSizeAgainstDataSource(
        profile.spec.list.pageSize,
        effectiveDataSource,
        'spec.list.pageSize',
        issues,
      );
    }
    if (
      profile.spec.list?.defaultSort &&
      effectiveDataSource.maxSortFields !== undefined &&
      profile.spec.list.defaultSort.length > effectiveDataSource.maxSortFields
    ) {
      addIssue(
        issues,
        'too-many-data-source-sort-fields',
        'spec.list.defaultSort',
        'Default sort exceeds the compiled data source limit',
      );
    }
  }
  const actions = new Map(capability.actions.map((action) => [action.id, action]));
  for (const duplicate of duplicateIDs(
    (profile.spec.actions ?? []).map((action) => action.action),
  )) {
    addIssue(issues, 'duplicate-action', 'spec.actions', `Action ${duplicate} is configured twice`);
  }
  profile.spec.actions?.forEach((action, index) => {
    const definition = actions.get(action.action);
    if (!definition) {
      addIssue(
        issues,
        'unknown-action',
        `spec.actions[${index}].action`,
        `Unknown action ${action.action}`,
      );
      return;
    }
    if (action.placement && !definition.placements.includes(action.placement)) {
      addIssue(
        issues,
        'unsupported-action-placement',
        `spec.actions[${index}].placement`,
        `Action ${action.action} cannot be placed at ${action.placement}`,
      );
    }
    validateLocalizedText(action.label, `spec.actions[${index}].label`, issues);
    validateLocalizedText(action.confirm, `spec.actions[${index}].confirm`, issues);
    validateCondition(
      action.visibleWhen,
      new Set(fieldDefinitions.keys()),
      `spec.actions[${index}].visibleWhen`,
      issues,
    );
  });
  return issues;
}

function mergeFieldCollection(
  current: readonly PageFieldPresentation[],
  patches: readonly PageFieldPresentationPatch[] | undefined,
): readonly PageFieldPresentation[] {
  if (!patches) return current.map(cloneField);
  const byField = new Map(current.map((field) => [field.field, cloneField(field)]));
  for (const patch of patches) {
    const existing = byField.get(patch.field);
    if (!existing) continue;
    byField.set(patch.field, {
      ...existing,
      label: mergeLocalized(existing.label, patch.label),
      component: patch.component ?? existing.component,
      order: patch.order ?? existing.order,
      hidden: patch.hidden ?? existing.hidden,
      width: patch.width ?? existing.width,
      span: patch.span ?? existing.span,
      placeholder: mergeLocalized(existing.placeholder, patch.placeholder),
      help: mergeLocalized(existing.help, patch.help),
      visibleWhen: patch.visibleWhen
        ? cloneCondition(patch.visibleWhen)
        : cloneCondition(existing.visibleWhen),
    });
  }
  return [...byField.values()];
}

function mergeActions(
  current: readonly PageActionPresentation[],
  patches: readonly PageActionPresentationPatch[] | undefined,
): readonly PageActionPresentation[] {
  if (!patches) return current.map(cloneAction);
  const byAction = new Map(current.map((action) => [action.action, cloneAction(action)]));
  for (const patch of patches) {
    const existing = byAction.get(patch.action);
    if (!existing) continue;
    byAction.set(patch.action, {
      ...existing,
      label: mergeLocalized(existing.label, patch.label),
      placement: patch.placement ?? existing.placement,
      order: patch.order ?? existing.order,
      hidden: patch.hidden ?? existing.hidden,
      confirm: mergeLocalized(existing.confirm, patch.confirm),
      visibleWhen: patch.visibleWhen
        ? cloneCondition(patch.visibleWhen)
        : cloneCondition(existing.visibleWhen),
    });
  }
  return [...byAction.values()];
}

function applyProfile(
  current: ResolvedPagePresentation,
  profile: AdminPagePresentationProfile,
): ResolvedPagePresentation {
  const spec = profile.spec;
  return {
    title: mergeLocalized(current.title, spec.title) ?? {},
    dataSource: spec.dataSource ?? current.dataSource,
    list: {
      columns: mergeFieldCollection(current.list.columns, spec.list?.columns),
      density: spec.list?.density ?? current.list.density,
      pageSize: spec.list?.pageSize ?? current.list.pageSize,
      defaultSort: spec.list?.defaultSort
        ? spec.list.defaultSort.map((sort) => ({ ...sort }))
        : current.list.defaultSort.map((sort) => ({ ...sort })),
    },
    search: {
      fields: mergeFieldCollection(current.search.fields, spec.search?.fields),
      collapsedByDefault: spec.search?.collapsedByDefault ?? current.search.collapsedByDefault,
    },
    form: {
      fields: mergeFieldCollection(current.form.fields, spec.form?.fields),
      columns: spec.form?.columns ?? current.form.columns,
    },
    detail: {
      fields: mergeFieldCollection(current.detail.fields, spec.detail?.fields),
      columns: spec.detail?.columns ?? current.detail.columns,
    },
    actions: mergeActions(current.actions, spec.actions),
  };
}

export function resolvePagePresentation(
  capability: PageCapabilityDefinition,
  layers: PresentationLayers,
  grantedPermissions: ReadonlySet<string>,
): PresentationResolution {
  const capabilityIssues = validatePageCapabilityDefinition(capability);
  if (capabilityIssues.length > 0) {
    throw new Error(
      `Invalid page capability ${capability.pageKey}: ${capabilityIssues
        .map((issue) => `${issue.path}: ${issue.message}`)
        .join('; ')}`,
    );
  }

  let presentation = clonePresentation(capability.defaultPresentation);
  const appliedLayers: PresentationLayer[] = [];
  const rejectedLayers: RejectedPresentationLayer[] = [];

  for (const layer of layerOrder) {
    const profile = layers[layer];
    if (!profile) continue;
    const issues = validatePagePresentationProfile(capability, profile);
    if (profile.metadata.scope.kind !== layer) {
      addIssue(
        issues,
        'scope-layer-mismatch',
        'metadata.scope.kind',
        `A ${layer} slot cannot contain a ${profile.metadata.scope.kind} profile`,
      );
    }
    if (issues.length > 0) {
      rejectedLayers.push({ layer, profileName: profile.metadata.name, issues });
      continue;
    }
    const candidate = applyProfile(presentation, profile);
    const candidateIssues = validateCompletePresentation(capability, candidate);
    if (candidateIssues.length > 0) {
      rejectedLayers.push({
        layer,
        profileName: profile.metadata.name,
        issues: candidateIssues,
      });
      continue;
    }
    presentation = candidate;
    appliedLayers.push(layer);
  }

  const dataSource = capability.dataSources.find(
    (candidate) => candidate.id === presentation.dataSource,
  );
  const authorized = Boolean(
    dataSource?.requiredPermissions.every((permission) => grantedPermissions.has(permission)),
  );
  const actionDefinitions = new Map(capability.actions.map((action) => [action.id, action]));
  const removedActionIds: string[] = [];
  const securedActions = presentation.actions.map((action) => {
    const definition = actionDefinitions.get(action.action);
    const permitted = Boolean(
      authorized &&
        definition?.requiredPermissions.every((permission) => grantedPermissions.has(permission)),
    );
    if (!permitted && !action.hidden) removedActionIds.push(action.action);
    return { ...cloneAction(action), hidden: action.hidden || !permitted };
  });

  return {
    presentation: { ...presentation, actions: securedActions },
    authorized,
    appliedLayers,
    rejectedLayers,
    removedActionIds: removedActionIds.sort(),
  };
}

function localize(value: LocalizedText | undefined, locale: AdminLocale, fallback: string): string {
  if (!value) return fallback;
  const otherLocale: AdminLocale = locale === 'zh-CN' ? 'en-US' : 'zh-CN';
  return value[locale]?.trim() || value[otherLocale]?.trim() || fallback;
}

function renderFields(
  fields: readonly PageFieldPresentation[],
  definitions: ReadonlyMap<string, PageCapabilityField>,
  locale: AdminLocale,
): readonly PageRenderField[] {
  return fields
    .filter((field) => !field.hidden)
    .sort((left, right) => left.order - right.order || left.field.localeCompare(right.field))
    .map((field) => {
      const definition = definitions.get(field.field);
      return {
        field: field.field,
        label: localize(field.label, locale, localize(definition?.label, locale, field.field)),
        component: field.component,
        order: field.order,
        width: field.width,
        span: field.span,
        placeholder: field.placeholder
          ? localize(field.placeholder, locale, field.field)
          : undefined,
        help: field.help ? localize(field.help, locale, field.field) : undefined,
        visibleWhen: cloneCondition(field.visibleWhen),
      };
    });
}

export function buildPageRenderModel(
  capability: PageCapabilityDefinition,
  resolution: PresentationResolution,
  locale: AdminLocale,
): PageRenderModel {
  const presentation = resolution.presentation;
  const fields = new Map(capability.fields.map((field) => [field.id, field]));
  const actions = presentation.actions
    .filter((action) => !action.hidden)
    .sort((left, right) => left.order - right.order || left.action.localeCompare(right.action))
    .map((action) => ({
      action: action.action,
      label: localize(action.label, locale, action.action),
      placement: action.placement,
      order: action.order,
      confirm: action.confirm ? localize(action.confirm, locale, action.action) : undefined,
      visibleWhen: cloneCondition(action.visibleWhen),
    }));

  return {
    pageKey: capability.pageKey,
    status: resolution.authorized ? 'ready' : 'permission-denied',
    title: localize(presentation.title, locale, capability.pageKey),
    dataSource: resolution.authorized ? presentation.dataSource : undefined,
    list: {
      columns: renderFields(presentation.list.columns, fields, locale),
      density: presentation.list.density,
      pageSize: presentation.list.pageSize,
      defaultSort: presentation.list.defaultSort.map((sort) => ({ ...sort })),
    },
    search: {
      fields: renderFields(presentation.search.fields, fields, locale),
      collapsedByDefault: presentation.search.collapsedByDefault,
    },
    form: {
      fields: renderFields(presentation.form.fields, fields, locale),
      columns: presentation.form.columns,
    },
    detail: {
      fields: renderFields(presentation.detail.fields, fields, locale),
      columns: presentation.detail.columns,
    },
    actions,
  };
}
