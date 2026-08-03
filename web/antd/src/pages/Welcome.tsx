import { PageContainer } from '@ant-design/pro-components';
import { Area } from '@ant-design/charts';
import { FormattedMessage, useModel, history } from '@umijs/max';
import { Avatar, Card, Col, Row, Statistic, Typography, Space, theme, Alert } from 'antd';
import React from 'react';
import { useMonitorData } from '@/hooks/useMonitorData';
import {
  UserOutlined,
  SettingOutlined,
  TeamOutlined,
  FileTextOutlined,
  SafetyOutlined,
  MenuOutlined,
} from '@ant-design/icons';

const { Title, Text } = Typography;

const QuickEntry: React.FC<{ icon: React.ReactNode; title: React.ReactNode; href: string }> = ({
  icon,
  title,
  href,
}) => {
  const { token } = theme.useToken();
  return (
    <div
      onClick={() => history.push(href)}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 12,
        padding: '12px 16px',
        borderRadius: 8,
        backgroundColor: token.colorBgContainer,
        boxShadow: token.boxShadowSecondary,
        cursor: 'pointer',
        transition: 'all 0.3s',
        minWidth: 180,
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.transform = 'translateY(-2px)';
        e.currentTarget.style.boxShadow = token.boxShadow;
        e.currentTarget.style.borderColor = token.colorPrimary;
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.transform = 'translateY(0)';
        e.currentTarget.style.boxShadow = token.boxShadowSecondary;
      }}
    >
      <div style={{ fontSize: 24, color: token.colorPrimary }}>{icon}</div>
      <Text style={{ color: token.colorText }}>{title}</Text>
    </div>
  );
};

const Welcome: React.FC = () => {
  const { token } = theme.useToken();
  const { initialState } = useModel('@@initialState');
  const { monitorData, historyData, loading, error } = useMonitorData();

  const currentUser = initialState?.currentUser;
  const hour = new Date().getHours();
  const greetingKey =
    hour < 6
      ? 'pages.welcome.greeting.night'
      : hour < 12
      ? 'pages.welcome.greeting.morning'
      : hour < 18
      ? 'pages.welcome.greeting.afternoon'
      : 'pages.welcome.greeting.evening';

  const cpuConfig = {
    data: historyData,
    xField: 'time',
    yField: 'cpu',
    smooth: true,
    animation: false,
    areaStyle: { fillOpacity: 0.6 },
    meta: { cpu: { alias: 'CPU %', max: 100 } },
  };

  const memoryConfig = {
    data: historyData,
    xField: 'time',
    yField: 'memory',
    smooth: true,
    animation: false,
    areaStyle: { fillOpacity: 0.6 },
    meta: { memory: { alias: 'Memory %', max: 100 } },
  };

  const memoryPercent = monitorData?.memoryUsagePercent?.toFixed(2) || '0.00';
  const diskPercent = monitorData?.diskUsagePercent?.toFixed(2) || '0.00';

  const quickEntries = [
    {
      icon: <UserOutlined />,
      title: <FormattedMessage id="menu.origination.user" defaultMessage="用户管理" />,
      href: '/users',
    },
    {
      icon: <TeamOutlined />,
      title: <FormattedMessage id="menu.origination.department" defaultMessage="部门管理" />,
      href: '/departments',
    },
    {
      icon: <SafetyOutlined />,
      title: <FormattedMessage id="menu.authority.role" defaultMessage="角色管理" />,
      href: '/role',
    },
    {
      icon: <MenuOutlined />,
      title: <FormattedMessage id="menu.authority.menu" defaultMessage="菜单管理" />,
      href: '/menu',
    },
    {
      icon: <SettingOutlined />,
      title: <FormattedMessage id="menu.system" defaultMessage="系统设置" />,
      href: '/app-config',
    },
    {
      icon: <FileTextOutlined />,
      title: <FormattedMessage id="menu.system.log" defaultMessage="日志管理" />,
      href: '/log',
    },
  ];

  return (
    <PageContainer>
      {error && (
        <Alert
          message={
            <FormattedMessage id="pages.monitor.error.title" defaultMessage="监控数据获取失败" />
          }
          description={
            <FormattedMessage
              id="pages.monitor.error.description"
              defaultMessage="无法获取系统监控数据，请检查服务是否正常运行"
            />
          }
          type="error"
          closable
          style={{ marginBottom: 16, borderRadius: 8 }}
        />
      )}

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <Card style={{ borderRadius: 8 }}>
            <Space align="start" size={16}>
              <Avatar
                size={64}
                src={currentUser?.avatar || undefined}
                icon={<UserOutlined />}
                style={{ backgroundColor: token.colorPrimary }}
              />
              <div>
                <Title level={4} style={{ margin: 0, marginBottom: 8 }}>
                  <FormattedMessage id={greetingKey} defaultMessage="你好" />，
                  {currentUser?.name || '用户'}
                </Title>
                <Text type="secondary">
                  <FormattedMessage
                    id="pages.welcome.lastLogin"
                    defaultMessage="欢迎使用 mss-boot-admin 管理系统"
                  />
                </Text>
              </div>
            </Space>
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card style={{ borderRadius: 8, height: '100%' }}>
            <Row gutter={16}>
              <Col span={8}>
                <Statistic
                  title="CPU"
                  value={monitorData?.cpuUsage || 0}
                  suffix="%"
                  valueStyle={{ fontSize: 20 }}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title={<FormattedMessage id="pages.monitor.memory" defaultMessage="内存" />}
                  value={memoryPercent}
                  suffix="%"
                  valueStyle={{ fontSize: 20 }}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title={<FormattedMessage id="pages.monitor.disk" defaultMessage="磁盘" />}
                  value={diskPercent}
                  suffix="%"
                  valueStyle={{ fontSize: 20 }}
                />
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>

      <Card
        title={<FormattedMessage id="pages.welcome.quickEntry" defaultMessage="快捷入口" />}
        style={{ borderRadius: 8, marginTop: 16 }}
      >
        <Space size={16} wrap>
          {quickEntries.map((entry, index) => (
            <QuickEntry key={index} {...entry} />
          ))}
        </Space>
      </Card>

      <Card loading={loading} style={{ borderRadius: 8, marginTop: 16 }}>
        <Title level={4}>
          <FormattedMessage id="pages.monitor.title" defaultMessage="系统监控" />
        </Title>
        <Row gutter={[16, 16]}>
          <Col xs={24} sm={12} md={6}>
            <Card>
              <Statistic
                title={<FormattedMessage id="pages.monitor.cpu" defaultMessage="CPU 使用率" />}
                value={monitorData?.cpuUsage?.toFixed(2) || '0.00'}
                suffix="%"
                valueStyle={{ color: (monitorData?.cpuUsage || 0) > 80 ? '#cf1322' : '#3f8600' }}
              />
              <div style={{ marginTop: 8, color: '#999' }}>
                {monitorData?.cpuPhysicalCore}{' '}
                <FormattedMessage id="pages.monitor.cpu.physicalCore" defaultMessage="物理核心" /> /{' '}
                {monitorData?.cpuLogicalCore}{' '}
                <FormattedMessage id="pages.monitor.cpu.logicalCore" defaultMessage="逻辑核心" />
              </div>
            </Card>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Card>
              <Statistic
                title={<FormattedMessage id="pages.monitor.memory" defaultMessage="内存使用" />}
                value={memoryPercent}
                suffix="%"
                valueStyle={{
                  color: parseFloat(memoryPercent as string) > 80 ? '#cf1322' : '#3f8600',
                }}
              />
              <div style={{ marginTop: 8, color: '#999' }}>
                {((monitorData?.memoryUsage || 0) / 1024 / 1024 / 1024).toFixed(2)} GB /{' '}
                {((monitorData?.memoryTotal || 0) / 1024 / 1024 / 1024).toFixed(2)} GB
              </div>
            </Card>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Card>
              <Statistic
                title={<FormattedMessage id="pages.monitor.disk" defaultMessage="磁盘使用" />}
                value={diskPercent}
                suffix="%"
                valueStyle={{
                  color: parseFloat(diskPercent as string) > 80 ? '#cf1322' : '#3f8600',
                }}
              />
              <div style={{ marginTop: 8, color: '#999' }}>
                {monitorData?.diskUsage?.toFixed(2) || '0.00'} GB /{' '}
                {monitorData?.diskTotal?.toFixed(2) || '0.00'} GB
              </div>
            </Card>
          </Col>
          <Col xs={24} sm={12} md={6}>
            <Card>
              <Statistic title="Goroutines" value={monitorData?.runtime?.goroutines || 0} />
              <div style={{ marginTop: 8, color: '#999' }}>
                <FormattedMessage id="pages.monitor.gc.count" defaultMessage="GC 次数" />:{' '}
                {monitorData?.runtime?.numGC || 0}
              </div>
            </Card>
          </Col>
        </Row>

        <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
          <Col xs={24} lg={12}>
            <Card
              title={
                <FormattedMessage id="pages.monitor.cpu.trend" defaultMessage="CPU 使用率趋势" />
              }
            >
              <Area {...cpuConfig} height={200} />
            </Card>
          </Col>
          <Col xs={24} lg={12}>
            <Card
              title={
                <FormattedMessage id="pages.monitor.memory.trend" defaultMessage="内存使用率趋势" />
              }
            >
              <Area {...memoryConfig} height={200} />
            </Card>
          </Col>
        </Row>

        <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
          <Col xs={24}>
            <Card
              title={<FormattedMessage id="pages.monitor.runtime" defaultMessage="运行时信息" />}
            >
              <Row gutter={16}>
                <Col span={6}>
                  <Statistic
                    title={<FormattedMessage id="pages.monitor.heap" defaultMessage="堆内存" />}
                    value={((monitorData?.runtime?.heapAlloc || 0) / 1024 / 1024).toFixed(2)}
                    suffix="MB"
                  />
                </Col>
                <Col span={6}>
                  <Statistic
                    title={
                      <FormattedMessage id="pages.monitor.gc.count" defaultMessage="GC 次数" />
                    }
                    value={monitorData?.runtime?.numGC || 0}
                  />
                </Col>
                <Col span={6}>
                  <Statistic title="Goroutines" value={monitorData?.runtime?.goroutines || 0} />
                </Col>
                <Col span={6}>
                  <Statistic
                    title={<FormattedMessage id="pages.monitor.uptime" defaultMessage="运行时间" />}
                    value={Math.floor((monitorData?.uptime || 0) / 3600)}
                    suffix={
                      <FormattedMessage id="pages.monitor.uptime.hour" defaultMessage="小时" />
                    }
                  />
                </Col>
              </Row>
            </Card>
          </Col>
        </Row>
      </Card>
    </PageContainer>
  );
};

export default Welcome;
