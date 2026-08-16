import { PageContainer } from '@ant-design/pro-components';
import { useIntl, useSearchParams } from '@umijs/max';
import { Grid, Tabs } from 'antd';
import AccessTokensPanel from '@/modules/account/AccessTokensPanel';
import NotificationSettingsPanel from '@/modules/account/NotificationSettingsPanel';
import OAuthBindingsPanel from '@/modules/account/OAuthBindingsPanel';
import ProfilePanel from '@/modules/account/ProfilePanel';
import SecurityPanel from '@/modules/account/SecurityPanel';
import ThemeSettingsEditor from '@/modules/theme/ThemeSettingsEditor';

export default function AccountSettingsPage() {
  const intl = useIntl();
  const screens = Grid.useBreakpoint();
  const [searchParams, setSearchParams] = useSearchParams();
  const validTabs = new Set([
    'profile',
    'security',
    'tokens',
    'connections',
    'notifications',
    'theme',
  ]);
  const requestedTab = searchParams.get('tab') ?? searchParams.get('key') ?? 'profile';
  const activeTab = validTabs.has(requestedTab) ? requestedTab : 'profile';

  return (
    <PageContainer title={intl.formatMessage({ id: 'pages.accountSettings.title' })}>
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
            key: 'profile',
            label: intl.formatMessage({ id: 'account.tabs.profile' }),
            children: <ProfilePanel />,
          },
          {
            key: 'security',
            label: intl.formatMessage({ id: 'account.tabs.security' }),
            children: <SecurityPanel />,
          },
          {
            key: 'tokens',
            label: intl.formatMessage({ id: 'account.tabs.tokens' }),
            children: <AccessTokensPanel />,
          },
          {
            key: 'connections',
            label: intl.formatMessage({ id: 'account.tabs.connections' }),
            children: <OAuthBindingsPanel />,
          },
          {
            key: 'notifications',
            label: intl.formatMessage({ id: 'account.tabs.notifications' }),
            children: <NotificationSettingsPanel />,
          },
          {
            key: 'theme',
            label: intl.formatMessage({ id: 'account.tabs.theme' }),
            children: <ThemeSettingsEditor scope="user" />,
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
