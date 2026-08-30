import type { InputProps, InputRef, RefSelectProps, SelectProps, SwitchProps } from 'antd';
import { Input, Select, Switch, Tag, Typography } from 'antd';
import { type AriaAttributes, type ForwardedRef, forwardRef, type ReactNode } from 'react';

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

interface PresentationFieldControlProps extends AriaAttributes {
  component: string;
  options?: readonly PresentationSelectOption[];
  placeholder?: string;
  maxLength?: number;
  allLabel: ReactNode;
  enabledLabel: ReactNode;
  disabledLabel: ReactNode;
  id?: string;
  value?: unknown;
  checked?: boolean;
  disabled?: boolean;
  name?: string;
  onBlur?: (...args: unknown[]) => void;
  onChange?: (...args: unknown[]) => void;
}

export type PresentationFieldControlRef = InputRef | RefSelectProps | HTMLButtonElement;

/**
 * Keeps the trusted presentation component registry inside the compiled
 * bundle while preserving Ant Design Form.Item's custom-control contract.
 * Form.Item clones its direct child with id, value/checked, handlers, aria
 * state, and a focus ref; every supported concrete control must receive them.
 */
export const PresentationFieldControl = forwardRef<
  PresentationFieldControlRef,
  PresentationFieldControlProps
>(
  (
    {
      component,
      options = [],
      placeholder,
      maxLength,
      allLabel,
      enabledLabel,
      disabledLabel,
      ...controlProps
    },
    ref,
  ) => {
    switch (component) {
      case 'input':
        return (
          <Input
            {...(controlProps as unknown as InputProps)}
            allowClear
            autoComplete="off"
            maxLength={maxLength}
            placeholder={placeholder}
            ref={ref as ForwardedRef<InputRef>}
          />
        );
      case 'email-input':
        return (
          <Input
            {...(controlProps as unknown as InputProps)}
            allowClear
            autoComplete="email"
            maxLength={maxLength}
            placeholder={placeholder}
            ref={ref as ForwardedRef<InputRef>}
            type="email"
          />
        );
      case 'select':
        return (
          <Select
            {...(controlProps as unknown as SelectProps<string>)}
            options={[...options]}
            placeholder={placeholder}
            ref={ref as ForwardedRef<RefSelectProps>}
            virtual={false}
          />
        );
      case 'boolean-filter':
        return (
          <Select
            {...(controlProps as unknown as SelectProps<string>)}
            options={[
              { value: 'all', label: allLabel },
              { value: 'true', label: enabledLabel },
              { value: 'false', label: disabledLabel },
            ]}
            placeholder={placeholder}
            ref={ref as ForwardedRef<RefSelectProps>}
            virtual={false}
          />
        );
      case 'switch':
        return (
          <Switch
            {...(controlProps as unknown as SwitchProps)}
            ref={ref as ForwardedRef<HTMLButtonElement>}
          />
        );
      default:
        return null;
    }
  },
);

PresentationFieldControl.displayName = 'PresentationFieldControl';
