import { QuestionCircleOutlined } from '@ant-design/icons';
import { SelectLang as UmiSelectLang, useIntl } from '@umijs/max';
import React from 'react';

export type SiderTheme = 'light' | 'dark';

export const SelectLang = () => {
  return (
    <UmiSelectLang
      style={{
        padding: 4,
      }}
    />
  );
};

export const Question = () => {
  const intl = useIntl();

  return (
    <a
      href="https://docs.mss-boot-io.top"
      target="_blank"
      rel="noreferrer"
      aria-label={intl.formatMessage({
        id: 'app.documentation',
        defaultMessage: 'Documentation',
      })}
      style={{
        display: 'flex',
        alignItems: 'center',
        height: 26,
        color: 'inherit',
      }}
    >
      <QuestionCircleOutlined />
    </a>
  );
};
