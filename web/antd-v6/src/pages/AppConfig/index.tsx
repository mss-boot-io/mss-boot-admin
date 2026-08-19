import BasePanel from '@mss-admin-core/modules/app-config/BasePanel';
import EmailPanel from '@mss-admin-core/modules/app-config/EmailPanel';
import SecurityPanel from '@mss-admin-core/modules/app-config/SecurityPanel';
import StoragePanel from '@mss-admin-core/modules/app-config/StoragePanel';
import ThemeSettingsEditor from '@mss-admin-core/modules/theme/ThemeSettingsEditor';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { useIntl, useSearchParams } from '@umijs/max';
import { Grid, Tabs } from 'antd';

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
        more={{
          trigger: 'click',
          icon: (
            <span className="whitespace-nowrap px-1 text-sm">
              {intl.formatMessage({ id: 'settings.tabs.more' })}
            </span>
          ),
        }}
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
