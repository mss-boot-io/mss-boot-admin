import { SettingOutlined, SlidersOutlined, UserOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Link, useIntl, useModel } from '@umijs/max';
import { Avatar, Button, Space, Tag, Typography } from 'antd';
import MonitorOverview from '@/modules/monitor/MonitorOverview';
import { hasPermission } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';

export default function WorkplacePage() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const currentUser = initialState?.currentUser;
  const displayName = currentUser?.name ?? currentUser?.username ?? 'MSS User';

  return (
    <PageContainer
      content={
        <Space>
          <Avatar src={currentUser?.avatar} icon={<UserOutlined />} />
          <Typography.Text type="secondary">
            {currentUser?.signature || intl.formatMessage({ id: 'pages.workplace.subtitle' })}
          </Typography.Text>
        </Space>
      }
      extra={currentUser?.role?.name ? <Tag color="blue">{currentUser.role.name}</Tag> : undefined}
      title={intl.formatMessage({ id: 'pages.workplace.greeting' }, { name: displayName })}
    >
      <ProCard
        className="mb-4"
        title={intl.formatMessage({ id: 'pages.workplace.quickLinks' })}
        variant="outlined"
      >
        <Space wrap>
          <Link prefetch to="/account/center">
            <Button icon={<UserOutlined />}>
              {intl.formatMessage({ id: 'menu.account-center' })}
            </Button>
          </Link>
          <Link prefetch to="/account/settings">
            <Button icon={<SlidersOutlined />}>
              {intl.formatMessage({ id: 'menu.account-settings' })}
            </Button>
          </Link>
          {hasPermission(currentUser, '/app-config') ? (
            <Link prefetch to="/app-config">
              <Button icon={<SettingOutlined />}>
                {intl.formatMessage({ id: 'menu.app-config' })}
              </Button>
            </Link>
          ) : null}
        </Space>
      </ProCard>
      <ProCard
        title={intl.formatMessage({ id: 'monitor.title' })}
        tooltip={intl.formatMessage({ id: 'monitor.authorityNotice' })}
        variant="outlined"
      >
        <MonitorOverview />
      </ProCard>
    </PageContainer>
  );
}
