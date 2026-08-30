import type {
  PageCapabilityField,
  PresentationCondition,
  PresentationPredicateOperator,
  PresentationScalar,
} from '@mss-admin-core/shared/presentation/contract';
import { Alert, Button, Input, Select, Space, Typography } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { usePresentationIntl } from './messages';

interface PresentationConditionEditorProps {
  condition?: PresentationCondition;
  fields: readonly PageCapabilityField[];
  onChange: (condition: PresentationCondition | undefined) => void;
  rawPath: string;
}

const baseOperators: readonly PresentationPredicateOperator[] = [
  'eq',
  'neq',
  'in',
  'not-in',
  'exists',
  'not-exists',
];
const orderedOperators: readonly PresentationPredicateOperator[] = ['gt', 'gte', 'lt', 'lte'];

function isSimpleCondition(
  condition: PresentationCondition | undefined,
): condition is Extract<PresentationCondition, { field: string }> {
  return Boolean(condition && 'field' in condition && 'operator' in condition);
}

function operatorsForField(field: PageCapabilityField | undefined) {
  if (
    field?.valueType === 'integer' ||
    field?.valueType === 'number' ||
    field?.valueType === 'date' ||
    field?.valueType === 'date-time'
  ) {
    return [...baseOperators, ...orderedOperators];
  }
  return [...baseOperators];
}

function defaultConditionValue(field: PageCapabilityField | undefined): PresentationScalar {
  if (field?.valueType === 'boolean') return false;
  if (field?.valueType === 'integer' || field?.valueType === 'number') return 0;
  return '';
}

function isScalarArray(
  value: PresentationScalar | readonly PresentationScalar[] | undefined,
): value is readonly PresentationScalar[] {
  return Array.isArray(value);
}

function valueForOperator(
  operator: PresentationPredicateOperator,
  field: PageCapabilityField | undefined,
  current: PresentationScalar | readonly PresentationScalar[] | undefined,
): PresentationScalar | readonly PresentationScalar[] | undefined {
  if (operator === 'exists' || operator === 'not-exists') return undefined;
  const fallback = defaultConditionValue(field);
  if (operator === 'in' || operator === 'not-in') {
    if (isScalarArray(current)) return current;
    return [current ?? fallback];
  }
  if (isScalarArray(current)) return current[0] ?? fallback;
  return current ?? fallback;
}

function ConditionValueInput({
  operator,
  value,
  onChange,
}: {
  operator: PresentationPredicateOperator;
  value: PresentationScalar | readonly PresentationScalar[] | undefined;
  onChange: (value: PresentationScalar | readonly PresentationScalar[]) => void;
}) {
  const intl = usePresentationIntl();
  const [text, setText] = useState(() => JSON.stringify(value ?? ''));
  const [invalid, setInvalid] = useState(false);
  useEffect(() => {
    setText(JSON.stringify(value ?? ''));
    setInvalid(false);
  }, [value]);

  const commit = () => {
    try {
      const parsed = JSON.parse(text) as unknown;
      const expectsArray = operator === 'in' || operator === 'not-in';
      const validScalar =
        parsed === null ||
        typeof parsed === 'boolean' ||
        typeof parsed === 'number' ||
        typeof parsed === 'string';
      const validArray =
        Array.isArray(parsed) &&
        parsed.length <= 50 &&
        parsed.every(
          (item) =>
            item === null ||
            typeof item === 'boolean' ||
            typeof item === 'number' ||
            typeof item === 'string',
        );
      if ((expectsArray && !validArray) || (!expectsArray && !validScalar)) {
        setInvalid(true);
        return;
      }
      setInvalid(false);
      onChange(parsed as PresentationScalar | readonly PresentationScalar[]);
    } catch {
      setInvalid(true);
    }
  };

  return (
    <Space orientation="vertical" size={2} className="w-full">
      <Input
        aria-label={intl.formatMessage({ id: 'presentation.visual.condition.value' })}
        maxLength={4_096}
        status={invalid ? 'error' : undefined}
        value={text}
        onBlur={commit}
        onChange={(event) => setText(event.target.value)}
        onPressEnter={commit}
      />
      {invalid ? (
        <Typography.Text type="danger">
          {intl.formatMessage({ id: 'presentation.visual.condition.value.invalid' })}
        </Typography.Text>
      ) : null}
    </Space>
  );
}

export default function PresentationConditionEditor({
  condition,
  fields,
  onChange,
  rawPath,
}: PresentationConditionEditorProps) {
  const intl = usePresentationIntl();
  const simple = isSimpleCondition(condition) ? condition : undefined;
  const selectedField = fields.find((field) => field.id === simple?.field) ?? fields[0];
  const operators = useMemo(() => operatorsForField(selectedField), [selectedField]);

  if (!fields.length) return null;
  if (condition && !simple) {
    return (
      <Alert
        action={
          <Space wrap>
            <Button
              size="small"
              onClick={() => {
                const field = fields[0];
                if (!field) return;
                onChange({ field: field.id, operator: 'eq', value: defaultConditionValue(field) });
              }}
            >
              {intl.formatMessage({ id: 'presentation.visual.condition.replace' })}
            </Button>
            <Button size="small" onClick={() => onChange(undefined)}>
              {intl.formatMessage({ id: 'presentation.visual.inherit' })}
            </Button>
          </Space>
        }
        description={rawPath}
        title={intl.formatMessage({ id: 'presentation.visual.condition.compound' })}
        type="info"
      />
    );
  }
  if (!simple) {
    return (
      <Button
        size="small"
        onClick={() => {
          const field = fields[0];
          if (!field) return;
          onChange({ field: field.id, operator: 'eq', value: defaultConditionValue(field) });
        }}
      >
        {intl.formatMessage({ id: 'presentation.visual.condition.add' })}
      </Button>
    );
  }

  const presence = simple.operator === 'exists' || simple.operator === 'not-exists';
  return (
    <Space orientation="vertical" size="small" className="w-full">
      <Space wrap align="start" className="w-full">
        <Select
          aria-label={intl.formatMessage({ id: 'presentation.visual.condition.field' })}
          options={fields.map((field) => ({ value: field.id, label: field.id }))}
          value={simple.field}
          onChange={(fieldID) => {
            const field = fields.find((item) => item.id === fieldID);
            const allowed = operatorsForField(field);
            const operator = allowed.includes(simple.operator) ? simple.operator : 'eq';
            const value = valueForOperator(operator, field, undefined);
            onChange({
              field: fieldID,
              operator,
              ...(value !== undefined ? { value } : {}),
            });
          }}
        />
        <Select
          aria-label={intl.formatMessage({ id: 'presentation.visual.condition.operator' })}
          options={operators.map((operator) => ({ value: operator, label: operator }))}
          value={simple.operator}
          onChange={(operator) => {
            const value = valueForOperator(operator, selectedField, simple.value);
            onChange({
              field: simple.field,
              operator,
              ...(value !== undefined ? { value } : {}),
            });
          }}
        />
        <Button size="small" onClick={() => onChange(undefined)}>
          {intl.formatMessage({ id: 'presentation.visual.inherit' })}
        </Button>
      </Space>
      {!presence ? (
        <ConditionValueInput
          operator={simple.operator}
          value={simple.value}
          onChange={(value) => onChange({ ...simple, value })}
        />
      ) : null}
    </Space>
  );
}
