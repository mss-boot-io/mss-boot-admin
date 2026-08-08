import { SearchOutlined } from '@ant-design/icons';
import type { InputRef } from 'antd';
import { AutoComplete, Button, Input } from 'antd';
import type { AutoCompleteProps } from 'antd/es/auto-complete';
import classNames from 'classnames';
import useMergedState from 'rc-util/es/hooks/useMergedState';
import React, { useEffect, useRef } from 'react';
import styles from './index.less';
import { useIntl } from '@umijs/max';

export type HeaderSearchProps = {
  onSearch?: (value?: string) => void;
  onSelect?: AutoCompleteProps['onSelect'];
  onChange?: (value?: string) => void;
  onVisibleChange?: (b: boolean) => void;
  className?: string;
  placeholder?: string;
  triggerLabel?: string;
  notFoundContent?: React.ReactNode;
  options?: AutoCompleteProps['options'];
  defaultVisible?: boolean;
  visible?: boolean;
  defaultValue?: string;
  value?: string;
};

const HeaderSearch: React.FC<HeaderSearchProps> = (props) => {
  const {
    className,
    defaultValue,
    notFoundContent,
    onSelect,
    onVisibleChange,
    placeholder,
    triggerLabel,
    visible,
    defaultVisible,
    ...restProps
  } = props;

  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const inputRef = useRef<InputRef | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);

  const [value, setValue] = useMergedState<string | undefined>(defaultValue, {
    value: props.value,
    onChange: props.onChange,
  });

  const [searchMode, setSearchMode] = useMergedState(defaultVisible ?? false, {
    value: visible,
    onChange: onVisibleChange,
  });

  const inputClass = classNames(styles.input, {
    [styles.show]: searchMode,
  });
  const placeholderText = intl.formatMessage({
    id: placeholder || 'component.search.placeholder',
    defaultMessage: 'Search menus',
  });
  const triggerText = intl.formatMessage({
    id: triggerLabel || 'component.search.open',
    defaultMessage: 'Open menu search',
  });

  useEffect(() => {
    if (searchMode) inputRef.current?.focus();
  }, [searchMode]);

  return (
    <div
      className={classNames(className, styles.headerSearch, { [styles.open]: searchMode })}
      role="search"
    >
      <Button
        ref={triggerRef}
        type="text"
        className={styles.trigger}
        icon={<SearchOutlined />}
        aria-label={triggerText}
        aria-expanded={searchMode}
        onMouseDown={(event) => {
          if (searchMode) event.preventDefault();
        }}
        onClick={() => {
          const nextOpen = !searchMode;
          setSearchMode(nextOpen);
          if (!nextOpen) triggerRef.current?.focus();
        }}
      />
      <AutoComplete
        key="AutoComplete"
        className={inputClass}
        value={value}
        options={restProps.options}
        onChange={(completeValue) => setValue(completeValue)}
        onSelect={(selectedValue, option) => {
          onSelect?.(selectedValue, option);
          setSearchMode(false);
          triggerRef.current?.focus();
        }}
        tabIndex={searchMode ? 0 : -1}
        open={searchMode && Boolean(value?.trim())}
        filterOption={false}
        notFoundContent={notFoundContent}
      >
        <Input
          size="small"
          ref={inputRef}
          aria-label={placeholderText}
          placeholder={placeholderText}
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              setSearchMode(false);
              triggerRef.current?.focus();
              return;
            }
            if (e.key === 'Enter') {
              if (restProps.onSearch) {
                restProps.onSearch(value);
              }
            }
          }}
          onBlur={() => {
            setSearchMode(false);
          }}
        />
      </AutoComplete>
    </div>
  );
};

export default HeaderSearch;
