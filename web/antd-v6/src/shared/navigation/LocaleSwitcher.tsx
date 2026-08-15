import TranslationOutlined from '@ant-design/icons/TranslationOutlined';
import { getLocale, setLocale, useIntl } from '@umijs/max';
import { Button, Dropdown, type MenuProps } from 'antd';

const localeItems: NonNullable<MenuProps['items']> = [
  { key: 'zh-CN', label: '简体中文' },
  { key: 'en-US', label: 'English' },
];

export default function LocaleSwitcher() {
  const intl = useIntl();
  const currentLocale = getLocale();
  const label = intl.formatMessage({ id: 'actions.switchLanguage' });

  return (
    <Dropdown
      menu={{
        items: localeItems,
        selectedKeys: [currentLocale],
        onClick: ({ key }) => {
          if (key !== currentLocale) setLocale(key);
        },
      }}
      placement="bottomRight"
      trigger={['click']}
    >
      <Button
        aria-label={label}
        icon={<TranslationOutlined />}
        shape="circle"
        title={label}
        type="text"
      />
    </Dropdown>
  );
}
