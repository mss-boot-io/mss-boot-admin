import { PageContainer } from '@ant-design/pro-components';
import { useIntl, useSearchParams } from '@umijs/max';
import { Grid, Tabs } from 'antd';
import BasePanel from '@/modules/app-config/BasePanel';
import EmailPanel from '@/modules/app-config/EmailPanel';
import SecurityPanel from '@/modules/app-config/SecurityPanel';
import StoragePanel from '@/modules/app-config/StoragePanel';
import ThemeSettingsEditor from '@/modules/theme/ThemeSettingsEditor';

export default function AppConfigPage() {
  const intl = useIntl();
  const screens = Grid.useBreakpoint();
  const [searchParams, setSearchParams] = useSearchParams();
  const validTabs = new Set(['base', 'security', 'storage', 'email', 'theme']);
  const requestedTab = searchParams.get('tab') ?? searchParams.get('key') ?? 'base';
  const activeTab = validTabs.has(requestedTab) ? requestedTab : 'base';
  return (
    <PageContainer title={intl.formatMessage({ id: 'pages.appConfig.title' })}>
      <Tabs
        activeKey={activeTab}
        tabPlacement={screens.lg ? 'start' : 'top'}
        destroyOnHidden
        items={[
          {
            key: 'base',
            label: intl.formatMessage({ id: 'pages.appConfig.tabs.base' }),
            children: <BasePanel />,
          },
          {
            key: 'security',
            label: intl.formatMessage({ id: 'pages.appConfig.tabs.security' }),
            children: <SecurityPanel />,
          },
          {
            key: 'storage',
            label: intl.formatMessage({ id: 'pages.appConfig.tabs.storage' }),
            children: <StoragePanel />,
          },
          {
            key: 'email',
            label: intl.formatMessage({ id: 'pages.appConfig.tabs.email' }),
            children: <EmailPanel />,
          },
          {
            key: 'theme',
            label: intl.formatMessage({ id: 'pages.theme.title' }),
            children: <ThemeSettingsEditor scope="application" />,
          },
        ]}
        onChange={(tab) => {
          const next = new URLSearchParams(searchParams);
          next.delete('key');
          next.set('tab', tab);
          setSearchParams(next);
        }}
      />
    </PageContainer>
  );
}
