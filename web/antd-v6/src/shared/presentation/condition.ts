import type {
  PageCapabilityDefinition,
  PresentationCondition,
  PresentationScalar,
} from './contract';

export function buildPresentationConditionContext(
  values: Readonly<Record<string, unknown>>,
  fieldBindings: Readonly<Record<string, string>> = {},
): Readonly<Record<string, unknown>> {
  const context: Record<string, unknown> = Object.create(null);
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined) context[key] = value;
  }
  for (const [field, binding] of Object.entries(fieldBindings)) {
    if (Object.hasOwn(values, binding) && values[binding] !== undefined) {
      context[field] = values[binding];
    } else {
      delete context[field];
    }
  }
  return context;
}

function scalarEquals(left: unknown, right: PresentationScalar): boolean {
  return (
    (left === null ||
      typeof left === 'string' ||
      typeof left === 'number' ||
      typeof left === 'boolean') &&
    Object.is(left, right)
  );
}

function orderedValue(value: unknown, valueType: string): number | undefined {
  if ((valueType === 'integer' || valueType === 'number') && typeof value === 'number') {
    return Number.isFinite(value) ? value : undefined;
  }
  if ((valueType === 'date' || valueType === 'date-time') && typeof value === 'string') {
    const parsed = Date.parse(value);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}

export function evaluatePresentationCondition(
  condition: PresentationCondition | undefined,
  values: Readonly<Record<string, unknown>>,
  capability: PageCapabilityDefinition,
): boolean {
  if (!condition) return true;
  if ('all' in condition) {
    return condition.all.every((child) => evaluatePresentationCondition(child, values, capability));
  }
  if ('any' in condition) {
    return condition.any.some((child) => evaluatePresentationCondition(child, values, capability));
  }
  if ('not' in condition) return !evaluatePresentationCondition(condition.not, values, capability);

  const present = Object.hasOwn(values, condition.field);
  if (condition.operator === 'exists') return present;
  if (condition.operator === 'not-exists') return !present;
  if (!present) return false;
  const actual = values[condition.field];
  if (condition.operator === 'eq')
    return scalarEquals(actual, condition.value as PresentationScalar);
  if (condition.operator === 'neq')
    return !scalarEquals(actual, condition.value as PresentationScalar);
  if (condition.operator === 'in' || condition.operator === 'not-in') {
    const expected = Array.isArray(condition.value) ? condition.value : [];
    const contained = expected.some((candidate) => scalarEquals(actual, candidate));
    return condition.operator === 'in' ? contained : !contained;
  }
  const field = capability.fields.find((candidate) => candidate.id === condition.field);
  const left = orderedValue(actual, field?.valueType ?? '');
  const right = orderedValue(condition.value, field?.valueType ?? '');
  if (left === undefined || right === undefined) return false;
  if (condition.operator === 'gt') return left > right;
  if (condition.operator === 'gte') return left >= right;
  if (condition.operator === 'lt') return left < right;
  return condition.operator === 'lte' && left <= right;
}
