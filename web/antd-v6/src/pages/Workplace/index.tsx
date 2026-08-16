import ApartmentOutlined from '@ant-design/icons/ApartmentOutlined';
import FileTextOutlined from '@ant-design/icons/FileTextOutlined';
import MenuOutlined from '@ant-design/icons/MenuOutlined';
import SafetyCertificateOutlined from '@ant-design/icons/SafetyCertificateOutlined';
import SettingOutlined from '@ant-design/icons/SettingOutlined';
import SlidersOutlined from '@ant-design/icons/SlidersOutlined';
import TeamOutlined from '@ant-design/icons/TeamOutlined';
import UserOutlined from '@ant-design/icons/UserOutlined';
import { ProCard } from '@ant-design/pro-components';
import { Link, useIntl, useModel } from '@umijs/max';
import { Avatar, Button, Col, Row, Space, Tag, Typography } from 'antd';
import MonitorOverview from '@/modules/monitor/MonitorOverview';
import { hasPermission } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageContainer } from '@/shared/design-system/PageContainer';

export default function WorkplacePage() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const currentUser = initialState?.currentUser;
  const displayName = currentUser?.name ?? currentUser?.username ?? 'MSS User';
  const quickLinks = [
    {
      path: '/account/center',
      icon: <UserOutlined />,
      label: intl.formatMessage({ id: 'menu.account-center' }),
      visible: true,
    },
    {
      path: '/account/settings',
      icon: <SlidersOutlined />,
      label: intl.formatMessage({ id: 'menu.account-settings' }),
      visible: true,
    },
    {
      path: '/users',
      icon: <TeamOutlined />,
      label: intl.formatMessage({ id: 'menu.users' }),
      visible: hasPermission(currentUser, '/users'),
    },
    {
      path: '/departments',
      icon: <ApartmentOutlined />,
      label: intl.formatMessage({ id: 'menu.departments' }),
      visible: hasPermission(currentUser, '/departments'),
    },
    {
      path: '/role',
      icon: <SafetyCertificateOutlined />,
      label: intl.formatMessage({ id: 'menu.role' }),
      visible: hasPermission(currentUser, '/role'),
    },
    {
      path: '/menu',
      icon: <MenuOutlined />,
      label: intl.formatMessage({ id: 'menu.menu-management' }),
      visible: hasPermission(currentUser, '/menu'),
    },
    {
      path: '/app-config',
      icon: <SettingOutlined />,
      label: intl.formatMessage({ id: 'menu.app-config' }),
      visible: hasPermission(currentUser, '/app-config'),
    },
    {
      path: '/log',
      icon: <FileTextOutlined />,
      label: intl.formatMessage({ id: 'menu.system-log' }),
      visible: hasPermission(currentUser, '/log'),
    },
  ].filter((item) => item.visible);

  return (
    <PageContainer
      content={
        <Space>
          <Avatar src={currentUser?.avatar || undefined} icon={<UserOutlined />} />
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
        <Row gutter={[12, 12]}>
          {quickLinks.map((item) => (
            <Col key={item.path} xs={24} sm={12} lg={8} xl={6}>
              <Link className="block" prefetch to={item.path}>
                <Button block icon={item.icon} size="large">
                  {item.label}
                </Button>
              </Link>
            </Col>
          ))}
        </Row>
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
