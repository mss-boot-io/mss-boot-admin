import { Input, Select, Switch, Tag, Typography } from 'antd';
import type { ReactNode } from 'react';

export interface PresentationSelectOption {
  value: string;
  label: ReactNode;
  color?: string;
}

interface PresentationValueProps {
  component: string;
  value: unknown;
  options?: readonly PresentationSelectOption[];
  enabledLabel: ReactNode;
  disabledLabel: ReactNode;
  formatDate: (value: string) => string;
}

export function renderPresentationValue({
  component,
  value,
  options = [],
  enabledLabel,
  disabledLabel,
  formatDate,
}: PresentationValueProps): ReactNode {
  switch (component) {
    case 'boolean':
      return (
        <Tag color={value === true ? 'green' : 'default'}>
          {value === true ? enabledLabel : disabledLabel}
        </Tag>
      );
    case 'tag': {
      const option = options.find((candidate) => candidate.value === value);
      return option ? <Tag color={option.color}>{option.label}</Tag> : String(value ?? '—');
    }
    case 'date-time':
      return typeof value === 'string' && value ? formatDate(value) : '—';
    case 'copyable-code':
      return value === undefined || value === null || value === '' ? (
        '—'
      ) : (
        <Typography.Text copyable code>
          {String(value)}
        </Typography.Text>
      );
    case 'text':
      return value === undefined || value === null || value === '' ? '—' : String(value);
    default:
      return null;
  }
}

interface PresentationFieldControlProps {
  component: string;
  options?: readonly PresentationSelectOption[];
  placeholder?: string;
  maxLength?: number;
  allLabel: ReactNode;
  enabledLabel: ReactNode;
  disabledLabel: ReactNode;
}

export function PresentationFieldControl({
  component,
  options = [],
  placeholder,
  maxLength,
  allLabel,
  enabledLabel,
  disabledLabel,
}: PresentationFieldControlProps) {
  switch (component) {
    case 'input':
      return (
        <Input allowClear autoComplete="off" maxLength={maxLength} placeholder={placeholder} />
      );
    case 'email-input':
      return (
        <Input
          allowClear
          autoComplete="email"
          maxLength={maxLength}
          placeholder={placeholder}
          type="email"
        />
      );
    case 'select':
      return <Select options={[...options]} placeholder={placeholder} virtual={false} />;
    case 'boolean-filter':
      return (
        <Select
          options={[
            { value: 'all', label: allLabel },
            { value: 'true', label: enabledLabel },
            { value: 'false', label: disabledLabel },
          ]}
          placeholder={placeholder}
          virtual={false}
        />
      );
    case 'switch':
      return <Switch />;
    default:
      return null;
  }
}
