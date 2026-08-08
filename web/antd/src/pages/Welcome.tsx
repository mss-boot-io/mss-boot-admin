import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, Link, useIntl, useModel } from '@umijs/max';
import { Alert, Avatar, Button, Card, Col, Row, Space, Statistic, theme, Typography } from 'antd';
import React from 'react';
import MonitorTrend from '@/components/MonitorTrend';
import { useMonitorData } from '@/hooks/useMonitorData';
import { hasPermission } from '@/utils/authorization';
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
    <Link
      to={href}
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
        width: '100%',
        minWidth: 0,
        border: '1px solid transparent',
        textDecoration: 'none',
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.transform = 'translateY(-2px)';
        e.currentTarget.style.boxShadow = token.boxShadow;
        e.currentTarget.style.borderColor = token.colorPrimary;
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.transform = 'translateY(0)';
        e.currentTarget.style.boxShadow = token.boxShadowSecondary;
        e.currentTarget.style.borderColor = 'transparent';
      }}
    >
      <div style={{ fontSize: 24, color: token.colorPrimary }}>{icon}</div>
      <Text style={{ color: token.colorText }}>{title}</Text>
    </Link>
  );
};

const Welcome: React.FC = () => {
  const { token } = theme.useToken();
  const intl = useIntl();
  const { initialState } = useModel('@@initialState');
  const {
    monitorData,
    historyData,
    lastUpdated,
    loading,
    refreshing,
    stale,
    notReady,
    permissionDenied,
    error,
    refresh,
  } = useMonitorData();

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

  const memoryPercent = monitorData?.memoryUsagePercent;
  const diskPercent = monitorData?.diskUsagePercent;
  const formattedLastUpdated = lastUpdated
    ? new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'medium' }).format(
        lastUpdated,
      )
    : '--';
  const statusColor = (value: number | undefined) =>
    value === undefined ? token.colorText : value > 80 ? token.colorError : token.colorSuccess;

  const quickEntries = [
    {
      icon: <UserOutlined />,
      title: <FormattedMessage id="menu.origination.user" defaultMessage="用户管理" />,
      href: '/users',
      permission: '/users',
    },
    {
      icon: <TeamOutlined />,
      title: <FormattedMessage id="menu.origination.department" defaultMessage="部门管理" />,
      href: '/departments',
      permission: '/departments',
    },
    {
      icon: <SafetyOutlined />,
      title: <FormattedMessage id="menu.authority.role" defaultMessage="角色管理" />,
      href: '/role',
      permission: '/role',
    },
    {
      icon: <MenuOutlined />,
      title: <FormattedMessage id="menu.authority.menu" defaultMessage="菜单管理" />,
      href: '/menu',
      permission: '/menu',
    },
    {
      icon: <SettingOutlined />,
      title: <FormattedMessage id="menu.system" defaultMessage="系统设置" />,
      href: '/app-config',
      permission: '/app-config',
    },
    {
      icon: <FileTextOutlined />,
      title: <FormattedMessage id="menu.system.log" defaultMessage="日志管理" />,
      href: '/log',
      permission: '/log',
    },
  ].filter((entry) => hasPermission(currentUser, entry.permission));

  return (
    <PageContainer>
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
                <Title level={1} style={{ fontSize: 20, margin: 0, marginBottom: 8 }}>
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
                  value={monitorData?.cpuUsage?.toFixed(2) ?? '--'}
                  suffix="%"
                  valueStyle={{ fontSize: 20, whiteSpace: 'nowrap' }}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title={<FormattedMessage id="pages.monitor.memory" defaultMessage="内存" />}
                  value={memoryPercent?.toFixed(2) ?? '--'}
                  suffix="%"
                  valueStyle={{ fontSize: 20, whiteSpace: 'nowrap' }}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title={<FormattedMessage id="pages.monitor.disk" defaultMessage="磁盘" />}
                  value={diskPercent?.toFixed(2) ?? '--'}
                  suffix="%"
                  valueStyle={{ fontSize: 20, whiteSpace: 'nowrap' }}
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
        <Row gutter={[16, 16]}>
          {quickEntries.map(({ href, icon, title }) => (
            <Col key={href} xs={24} sm={12} lg={8}>
              <QuickEntry href={href} icon={icon} title={title} />
            </Col>
          ))}
        </Row>
      </Card>

      <Card loading={loading && !monitorData} style={{ borderRadius: 8, marginTop: 16 }}>
        <Title level={4}>
          <FormattedMessage id="pages.monitor.title" defaultMessage="系统监控" />
        </Title>
        {!loading && !monitorData && (
          <Alert
            showIcon
            type={permissionDenied ? 'warning' : notReady ? 'info' : 'error'}
            message={
              <FormattedMessage
                id={
                  permissionDenied
                    ? 'pages.monitor.permissionDenied.title'
                    : notReady
                    ? 'pages.monitor.notReady.title'
                    : 'pages.monitor.error.title'
                }
                defaultMessage={
                  permissionDenied
                    ? '暂无监控查看权限'
                    : notReady
                    ? '监控采样器正在准备'
                    : '系统状态加载失败'
                }
              />
            }
            description={
              <FormattedMessage
                id={
                  permissionDenied
                    ? 'pages.monitor.permissionDenied.description'
                    : notReady
                    ? 'pages.monitor.notReady.description'
                    : 'pages.monitor.error.description'
                }
                defaultMessage={
                  permissionDenied
                    ? '请联系管理员授予监控查看权限。'
                    : notReady
                    ? '首个样本生成后将自动显示。'
                    : '请稍后重试。'
                }
              />
            }
            action={
              <Button loading={refreshing} onClick={() => void refresh()} size="small">
                <FormattedMessage id="pages.monitor.retry" defaultMessage="重试" />
              </Button>
            }
          />
        )}

        {monitorData && (
          <>
            <Space direction="vertical" size={8} style={{ display: 'flex', marginBottom: 16 }}>
              {error && (
                <Alert
                  showIcon
                  type="error"
                  message={
                    <FormattedMessage
                      id="pages.monitor.error.title"
                      defaultMessage="系统状态加载失败"
                    />
                  }
                  description={
                    <FormattedMessage
                      id="pages.monitor.error.retainedDescription"
                      defaultMessage="刷新失败，当前继续显示最近一次成功采样的数据。"
                    />
                  }
                  action={
                    <Button loading={refreshing} onClick={() => void refresh()} size="small">
                      <FormattedMessage id="pages.monitor.retry" defaultMessage="重试" />
                    </Button>
                  }
                />
              )}
              {stale && !error && (
                <Alert
                  showIcon
                  type="warning"
                  message={
                    <FormattedMessage
                      id="pages.monitor.stale.title"
                      defaultMessage="监控数据暂时陈旧"
                    />
                  }
                  description={
                    <FormattedMessage
                      id="pages.monitor.stale.description"
                      defaultMessage="采样暂时失败，当前显示最近一次成功采样的数据。"
                    />
                  }
                />
              )}
              {historyData.length === 0 && !error && (
                <Alert
                  showIcon
                  type="info"
                  message={
                    <FormattedMessage
                      id="pages.monitor.emptyHistory"
                      defaultMessage="监控历史正在积累，趋势将在采样后显示。"
                    />
                  }
                />
              )}
              <Space size={[16, 4]} wrap>
                <Text type="secondary">
                  <FormattedMessage
                    id="pages.monitor.lastUpdated"
                    defaultMessage="最后更新：{time}"
                    values={{ time: formattedLastUpdated }}
                  />
                </Text>
                {monitorData.instanceId && (
                  <Text type="secondary">
                    <FormattedMessage
                      id="pages.monitor.instance"
                      defaultMessage="实例：{instance}"
                      values={{ instance: monitorData.instanceId }}
                    />
                  </Text>
                )}
                {refreshing && (
                  <Text type="secondary">
                    <FormattedMessage id="pages.monitor.refreshing" defaultMessage="正在刷新…" />
                  </Text>
                )}
              </Space>
            </Space>

            <Row gutter={[16, 16]}>
              <Col xs={24} sm={12} md={6}>
                <Card>
                  <Statistic
                    title={<FormattedMessage id="pages.monitor.cpu" defaultMessage="CPU 使用率" />}
                    value={monitorData.cpuUsage?.toFixed(2) ?? '--'}
                    suffix="%"
                    valueStyle={{ color: statusColor(monitorData.cpuUsage) }}
                  />
                  <div style={{ color: token.colorTextSecondary, marginTop: 8 }}>
                    {monitorData.cpuPhysicalCore ?? '--'}{' '}
                    <FormattedMessage
                      id="pages.monitor.cpu.physicalCore"
                      defaultMessage="物理核心"
                    />{' '}
                    / {monitorData.cpuLogicalCore ?? '--'}{' '}
                    <FormattedMessage
                      id="pages.monitor.cpu.logicalCore"
                      defaultMessage="逻辑核心"
                    />
                  </div>
                </Card>
              </Col>
              <Col xs={24} sm={12} md={6}>
                <Card>
                  <Statistic
                    title={<FormattedMessage id="pages.monitor.memory" defaultMessage="内存使用" />}
                    value={memoryPercent?.toFixed(2) ?? '--'}
                    suffix="%"
                    valueStyle={{ color: statusColor(memoryPercent) }}
                  />
                  <div style={{ color: token.colorTextSecondary, marginTop: 8 }}>
                    {monitorData.memoryUsage === undefined
                      ? '--'
                      : (monitorData.memoryUsage / 1024 / 1024 / 1024).toFixed(2)}{' '}
                    GB /{' '}
                    {monitorData.memoryTotal === undefined
                      ? '--'
                      : (monitorData.memoryTotal / 1024 / 1024 / 1024).toFixed(2)}{' '}
                    GB
                  </div>
                </Card>
              </Col>
              <Col xs={24} sm={12} md={6}>
                <Card>
                  <Statistic
                    title={<FormattedMessage id="pages.monitor.disk" defaultMessage="磁盘使用" />}
                    value={diskPercent?.toFixed(2) ?? '--'}
                    suffix="%"
                    valueStyle={{ color: statusColor(diskPercent) }}
                  />
                  <div style={{ color: token.colorTextSecondary, marginTop: 8 }}>
                    {monitorData.diskUsage?.toFixed(2) ?? '--'} GB /{' '}
                    {monitorData.diskTotal?.toFixed(2) ?? '--'} GB
                  </div>
                </Card>
              </Col>
              <Col xs={24} sm={12} md={6}>
                <Card>
                  <Statistic title="Goroutines" value={monitorData.runtime?.goroutines ?? 0} />
                  <div style={{ color: token.colorTextSecondary, marginTop: 8 }}>
                    <FormattedMessage id="pages.monitor.gc.count" defaultMessage="GC 次数" />:{' '}
                    {monitorData.runtime?.numGC ?? 0}
                  </div>
                </Card>
              </Col>
            </Row>

            <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
              <Col xs={24} lg={12}>
                <Card
                  title={
                    <FormattedMessage
                      id="pages.monitor.cpu.trend"
                      defaultMessage="CPU 使用率趋势"
                    />
                  }
                >
                  <MonitorTrend
                    data={historyData}
                    metric="cpu"
                    metricLabel={intl.formatMessage({
                      id: 'pages.monitor.cpu',
                      defaultMessage: 'CPU 使用率',
                    })}
                    emptyDescription={
                      <FormattedMessage
                        id="pages.monitor.emptyHistory"
                        defaultMessage="监控历史正在积累，趋势将在采样后显示。"
                      />
                    }
                  />
                </Card>
              </Col>
              <Col xs={24} lg={12}>
                <Card
                  title={
                    <FormattedMessage
                      id="pages.monitor.memory.trend"
                      defaultMessage="内存使用率趋势"
                    />
                  }
                >
                  <MonitorTrend
                    data={historyData}
                    metric="memory"
                    metricLabel={intl.formatMessage({
                      id: 'pages.monitor.memory',
                      defaultMessage: '内存使用',
                    })}
                    emptyDescription={
                      <FormattedMessage
                        id="pages.monitor.emptyHistory"
                        defaultMessage="监控历史正在积累，趋势将在采样后显示。"
                      />
                    }
                  />
                </Card>
              </Col>
            </Row>

            <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
              <Col xs={24}>
                <Card
                  title={
                    <FormattedMessage id="pages.monitor.runtime" defaultMessage="运行时信息" />
                  }
                >
                  <Row gutter={[16, 16]}>
                    <Col xs={12} sm={6}>
                      <Statistic
                        title={<FormattedMessage id="pages.monitor.heap" defaultMessage="堆内存" />}
                        value={((monitorData.runtime?.heapAlloc ?? 0) / 1024 / 1024).toFixed(2)}
                        suffix="MB"
                      />
                    </Col>
                    <Col xs={12} sm={6}>
                      <Statistic
                        title={
                          <FormattedMessage id="pages.monitor.gc.count" defaultMessage="GC 次数" />
                        }
                        value={monitorData.runtime?.numGC ?? 0}
                      />
                    </Col>
                    <Col xs={12} sm={6}>
                      <Statistic title="Goroutines" value={monitorData.runtime?.goroutines ?? 0} />
                    </Col>
                    <Col xs={12} sm={6}>
                      <Statistic
                        title={
                          <FormattedMessage id="pages.monitor.uptime" defaultMessage="运行时间" />
                        }
                        value={Math.floor((monitorData.uptime ?? 0) / 3600)}
                        suffix={
                          <FormattedMessage id="pages.monitor.uptime.hour" defaultMessage="小时" />
                        }
                      />
                    </Col>
                  </Row>
                </Card>
              </Col>
            </Row>
          </>
        )}
      </Card>
    </PageContainer>
  );
};

export default Welcome;
