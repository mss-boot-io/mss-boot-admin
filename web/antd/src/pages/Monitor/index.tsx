import { Area } from '@ant-design/charts';
import { Card, Col, Row, Statistic, Typography, Alert } from 'antd';
import React from 'react';
import { useMonitorData } from '@/hooks/useMonitorData';
import { FormattedMessage } from '@umijs/max';

const { Title } = Typography;

const Monitor: React.FC = () => {
  const { monitorData, historyData, loading, error } = useMonitorData();

  const cpuConfig = {
    data: historyData,
    xField: 'time',
    yField: 'cpu',
    smooth: true,
    animation: false,
    meta: {
      cpu: { alias: 'CPU 使用率 (%)', max: 100 },
    },
  };

  const memoryConfig = {
    data: historyData,
    xField: 'time',
    yField: 'memory',
    smooth: true,
    animation: false,
    meta: {
      memory: { alias: '内存使用率 (%)', max: 100 },
    },
  };

  if (loading) {
    return <Card loading />;
  }

  const memoryPercent =
    monitorData?.memoryTotal && monitorData?.memoryUsage
      ? ((monitorData.memoryUsage / monitorData.memoryTotal) * 100).toFixed(1)
      : 0;
  const diskPercent =
    monitorData?.diskTotal && monitorData?.diskUsage
      ? ((monitorData.diskUsage / monitorData.diskTotal) * 100).toFixed(1)
      : 0;

  return (
    <div>
      <Title level={4}>
        <FormattedMessage id="pages.monitor.title" defaultMessage="系统监控" />
      </Title>

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
        <Col xs={24} sm={12} md={6}>
          <Card>
            <Statistic
              title={<FormattedMessage id="pages.monitor.cpu" defaultMessage="CPU 使用率" />}
              value={monitorData?.cpuUsage || 0}
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
              {((monitorData?.memoryUsage || 0) / 1024 / 1024).toFixed(0)} MB /{' '}
              {((monitorData?.memoryTotal || 0) / 1024 / 1024).toFixed(0)} MB
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card>
            <Statistic
              title={<FormattedMessage id="pages.monitor.disk" defaultMessage="磁盘使用" />}
              value={diskPercent}
              suffix="%"
              valueStyle={{ color: parseFloat(diskPercent as string) > 80 ? '#cf1322' : '#3f8600' }}
            />
            <div style={{ marginTop: 8, color: '#999' }}>
              {monitorData?.diskUsage} GB / {monitorData?.diskTotal} GB
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
          <Card title={<FormattedMessage id="pages.monitor.runtime" defaultMessage="运行时信息" />}>
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
                  title={<FormattedMessage id="pages.monitor.gc.count" defaultMessage="GC 次数" />}
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
