import { Alert, Button, Card, Col, Row, Space, Statistic, theme, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import React from 'react';
import MonitorTrend from '@/components/MonitorTrend';
import { useMonitorData } from '@/hooks/useMonitorData';

const { Text, Title } = Typography;

const Monitor: React.FC = () => {
  const { token } = theme.useToken();
  const intl = useIntl();
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

  const memoryPercent = monitorData?.memoryUsagePercent;
  const diskPercent = monitorData?.diskUsagePercent;
  const formattedLastUpdated = lastUpdated
    ? new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'medium' }).format(
        lastUpdated,
      )
    : '--';

  if (loading && !monitorData) {
    return <Card loading />;
  }

  if (!monitorData) {
    return (
      <div>
        <Title level={4}>
          <FormattedMessage id="pages.monitor.title" defaultMessage="系统监控" />
        </Title>
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
      </div>
    );
  }

  const statusColor = (value: number | undefined) =>
    value === undefined ? token.colorText : value > 80 ? token.colorError : token.colorSuccess;

  return (
    <div>
      <Title level={4}>
        <FormattedMessage id="pages.monitor.title" defaultMessage="系统监控" />
      </Title>

      <Space direction="vertical" size={8} style={{ display: 'flex', marginBottom: 16 }}>
        {error && (
          <Alert
            showIcon
            type="error"
            message={
              <FormattedMessage id="pages.monitor.error.title" defaultMessage="系统状态加载失败" />
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
              <FormattedMessage id="pages.monitor.stale.title" defaultMessage="监控数据暂时陈旧" />
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
              <FormattedMessage id="pages.monitor.cpu.physicalCore" defaultMessage="物理核心" /> /{' '}
              {monitorData.cpuLogicalCore ?? '--'}{' '}
              <FormattedMessage id="pages.monitor.cpu.logicalCore" defaultMessage="逻辑核心" />
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
                : (monitorData.memoryUsage / 1024 / 1024).toFixed(0)}{' '}
              MB /{' '}
              {monitorData.memoryTotal === undefined
                ? '--'
                : (monitorData.memoryTotal / 1024 / 1024).toFixed(0)}{' '}
              MB
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
              <FormattedMessage id="pages.monitor.cpu.trend" defaultMessage="CPU 使用率趋势" />
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
              <FormattedMessage id="pages.monitor.memory.trend" defaultMessage="内存使用率趋势" />
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
          <Card title={<FormattedMessage id="pages.monitor.runtime" defaultMessage="运行时信息" />}>
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
                  title={<FormattedMessage id="pages.monitor.gc.count" defaultMessage="GC 次数" />}
                  value={monitorData.runtime?.numGC ?? 0}
                />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic title="Goroutines" value={monitorData.runtime?.goroutines ?? 0} />
              </Col>
              <Col xs={12} sm={6}>
                <Statistic
                  title={<FormattedMessage id="pages.monitor.uptime" defaultMessage="运行时间" />}
                  value={Math.floor((monitorData.uptime ?? 0) / 3600)}
                  suffix={<FormattedMessage id="pages.monitor.uptime.hour" defaultMessage="小时" />}
                />
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Monitor;
