import Storage from '@/pages/AppConfig/components/storage';
import { useResponsive } from '@/hooks/useResponsive';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl, useSearchParams } from '@umijs/max';
import { Tabs, type TabsProps } from 'antd';
import React from 'react';
import Base from './components/base';
import Security from './components/security';
import Theme from '../../components/MssBoot/theme';
import Email from '@/pages/AppConfig/components/email';

const APP_CONFIG_TAB_KEYS = ['base', 'security', 'storage', 'email', 'theme'] as const;
type AppConfigTabKey = (typeof APP_CONFIG_TAB_KEYS)[number];

const resolveAppConfigTabKey = (key: string | null): AppConfigTabKey =>
  APP_CONFIG_TAB_KEYS.includes(key as AppConfigTabKey) ? (key as AppConfigTabKey) : 'base';

const Settings: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();
  const { isMobile } = useResponsive();
  const [searchParams, setSearchParams] = useSearchParams();
  const key = resolveAppConfigTabKey(searchParams.get('key'));
  const menuMap: TabsProps['items'] = [
    {
      label: intl.formatMessage({
        id: 'pages.base.settings.title',
        defaultMessage: 'Basic Settings',
      }),
      key: 'base',
      children: <Base />,
    },
    {
      label: intl.formatMessage({
        id: 'pages.security.settings.title',
        defaultMessage: 'Security Settings',
      }),
      key: 'security',
      children: <Security />,
    },
    {
      label: intl.formatMessage({
        id: 'pages.storage.settings.title',
        defaultMessage: 'Storage Settings',
      }),
      key: 'storage',
      children: <Storage />,
    },
    {
      label: intl.formatMessage({
        id: 'pages.email.settings.title',
        defaultMessage: 'Email Settings',
      }),
      key: 'email',
      children: <Email />,
    },
    {
      label: intl.formatMessage({
        id: 'pages.theme.settings.title',
        defaultMessage: 'Theme Settings',
      }),
      key: 'theme',
      children: <Theme scope="application" />,
    },
  ];

  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.application.settings.title',
        defaultMessage: 'Application Settings',
      })}
    >
      <Tabs
        type="card"
        activeKey={key}
        tabPosition={isMobile ? 'top' : 'left'}
        items={menuMap}
        onTabClick={(nextKey: string) => setSearchParams({ key: nextKey })}
        style={{ width: '100%' }}
      />
    </PageContainer>
  );
};
export default Settings;
