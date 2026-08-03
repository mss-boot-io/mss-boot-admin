import { PageContainer } from '@ant-design/pro-components';
import { useIntl, useSearchParams } from '@umijs/max';
import { Tabs } from 'antd';
import React from 'react';
import BaseView from '../components/base';
import BindingView from '../components/binding';
import NotificationView from '../components/notification';
import SecurityView from '../components/security';
import Theme from '@/components/MssBoot/theme';
import AccessTokenView from '../components/AccessToken';
import styles from '@/styles/mobile.less';

const MobileSettings: React.FC = () => {
  const intl = useIntl();
  const [searchParams, setSearchParams] = useSearchParams();
  const key = searchParams.get('key') || 'base';

  const menuMap = [
    {
      label: intl.formatMessage({
        id: 'pages.base.settings.title',
        defaultMessage: 'Basic Settings',
      }),
      key: 'base',
      children: <BaseView />,
    },
    {
      label: intl.formatMessage({
        id: 'pages.accessToken.settings.title',
        defaultMessage: 'Access Token',
      }),
      key: 'accessToken',
      children: <AccessTokenView />,
    },
    {
      label: intl.formatMessage({
        id: 'pages.security.settings.title',
        defaultMessage: 'Security Settings',
      }),
      key: 'security',
      children: <SecurityView />,
    },
    {
      label: intl.formatMessage({
        id: 'pages.binding.settings.title',
        defaultMessage: 'Account Binding',
      }),
      key: 'binding',
      children: <BindingView />,
    },
    {
      label: intl.formatMessage({
        id: 'pages.notification.settings.title',
        defaultMessage: 'New Message Notification',
      }),
      key: 'notification',
      children: <NotificationView />,
    },
    {
      label: intl.formatMessage({
        id: 'pages.theme.settings.title',
        defaultMessage: 'Theme Settings',
      }),
      key: 'theme',
      children: <Theme />,
    },
  ];

  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.account.settings.title',
        defaultMessage: 'Personal Settings',
      })}
    >
      <div className={styles.mobileContainer}>
        <Tabs
          activeKey={key}
          tabPosition="top"
          items={menuMap}
          onTabClick={(k: string) => setSearchParams({ key: k })}
          style={{ marginBottom: 16 }}
        />
      </div>
    </PageContainer>
  );
};

export default MobileSettings;