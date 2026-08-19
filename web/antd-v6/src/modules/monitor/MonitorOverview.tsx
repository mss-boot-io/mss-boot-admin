import CloudServerOutlined from '@ant-design/icons/CloudServerOutlined';
import DashboardOutlined from '@ant-design/icons/DashboardOutlined';
import HddOutlined from '@ant-design/icons/HddOutlined';
import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import { ProCard } from '@ant-design/pro-components';
import { getRequestErrorMessage, getRequestStatus } from '@mss-admin-core/shared/api/errors';
import {
  PageEmpty,
  PageError,
  PageForbidden,
  PageLoading,
} from '@mss-admin-core/shared/design-system/PageState';
import { useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Col,
  Descriptions,
  Progress,
  Row,
  Space,
  Statistic,
  Tag,
  Typography,
  theme,
} from 'antd';
import type { MonitorSnapshot } from './contract';
import MonitorTrend from './MonitorTrend';
import { useMonitorSnapshot } from './query';

function formatBytes(value: number, locale: string): string {
  if (value <= 0) return '0 B';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(value / 1024 ** index)} ${units[index]}`;
}

function formatUptime(seconds: number, locale: string): string {
  const values = [
    { unit: 'day' as const, value: Math.floor(seconds / 86_400) },
    { unit: 'hour' as const, value: Math.floor((seconds % 86_400) / 3_600) },
    { unit: 'minute' as const, value: Math.floor((seconds % 3_600) / 60) },
  ].filter((entry) => entry.value > 0);
  if (values.length === 0) values.push({ unit: 'minute', value: 0 });
  return values
    .slice(0, 2)
    .map((entry) =>
      new Intl.NumberFormat(locale, {
        style: 'unit',
        unit: entry.unit,
        unitDisplay: 'short',
      }).format(entry.value),
    )
    .join(' ');
}

function percentColor(value: number, error: string, warning: string): string | undefined {
  if (value >= 90) return error;
  if (value >= 75) return warning;
  return undefined;
}

function ResourceMetric({
  icon,
  percent,
  title,
  detail,
}: {
  icon: React.ReactNode;
  percent: number;
  title: React.ReactNode;
  detail: React.ReactNode;
}) {
  const { token } = theme.useToken();
  const warning = percent >= 75 && percent < 90;
  return (
    <ProCard variant="outlined">
      <Statistic
        prefix={icon}
        precision={1}
        suffix="%"
        title={title}
        value={percent}
        styles={{ content: { color: percentColor(percent, token.colorError, token.colorWarning) } }}
      />
      <Progress
        aria-label={typeof title === 'string' ? title : undefined}
        percent={percent}
        showInfo={false}
        status={percent >= 90 ? 'exception' : 'normal'}
        strokeColor={warning ? token.colorWarning : undefined}
      />
      <Typography.Text className="mt-2 block" type="secondary">
        {detail}
      </Typography.Text>
    </ProCard>
  );
}

function MonitorContent({ snapshot }: { snapshot: MonitorSnapshot }) {
  const intl = useIntl();
  const { token } = theme.useToken();
  const locale = intl.locale || 'zh-CN';
  const collectedAt = new Intl.DateTimeFormat(locale, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  }).format(new Date(snapshot.collectedAt));

  return (
    <Space orientation="vertical" size="middle" className="w-full">
      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <ResourceMetric
            detail={intl.formatMessage(
              { id: 'monitor.cpu.cores' },
              { physical: snapshot.cpuPhysicalCore, logical: snapshot.cpuLogicalCore },
            )}
            icon={<DashboardOutlined />}
            percent={snapshot.cpuUsage}
            title={intl.formatMessage({ id: 'monitor.cpu.title' })}
          />
        </Col>
        <Col xs={24} md={8}>
          <ResourceMetric
            detail={intl.formatMessage(
              { id: 'monitor.memory.detail' },
              {
                used: formatBytes(snapshot.memoryUsage, locale),
                total: formatBytes(snapshot.memoryTotal, locale),
              },
            )}
            icon={<CloudServerOutlined />}
            percent={snapshot.memoryUsagePercent}
            title={intl.formatMessage({ id: 'monitor.memory.title' })}
          />
        </Col>
        <Col xs={24} md={8}>
          <ResourceMetric
            detail={intl.formatMessage(
              { id: 'monitor.disk.detail' },
              {
                used: new Intl.NumberFormat(locale, { maximumFractionDigits: 2 }).format(
                  snapshot.diskUsage,
                ),
                total: new Intl.NumberFormat(locale, { maximumFractionDigits: 2 }).format(
                  snapshot.diskTotal,
                ),
              },
            )}
            icon={<HddOutlined />}
            percent={snapshot.diskUsagePercent}
            title={intl.formatMessage({ id: 'monitor.disk.title' })}
          />
        </Col>
      </Row>

      <ProCard gutter={[16, 16]} wrap>
        <ProCard
          colSpan={{ xs: 24, lg: 12 }}
          title={intl.formatMessage({ id: 'monitor.cpu.trend' })}
          variant="outlined"
        >
          {snapshot.history.length > 0 ? (
            <MonitorTrend
              ariaLabel={intl.formatMessage({ id: 'monitor.cpu.trendLabel' })}
              points={snapshot.history.map((point) => ({
                timestamp: point.timestamp,
                value: point.cpuUsage,
              }))}
            />
          ) : (
            <PageEmpty description={intl.formatMessage({ id: 'monitor.history.empty' })} />
          )}
        </ProCard>
        <ProCard
          colSpan={{ xs: 24, lg: 12 }}
          title={intl.formatMessage({ id: 'monitor.memory.trend' })}
          variant="outlined"
        >
          {snapshot.history.length > 0 ? (
            <MonitorTrend
              ariaLabel={intl.formatMessage({ id: 'monitor.memory.trendLabel' })}
              color={token.colorSuccess}
              points={snapshot.history.map((point) => ({
                timestamp: point.timestamp,
                value: point.memoryUsagePercent,
              }))}
            />
          ) : (
            <PageEmpty description={intl.formatMessage({ id: 'monitor.history.empty' })} />
          )}
        </ProCard>
      </ProCard>

      <ProCard title={intl.formatMessage({ id: 'monitor.runtime.title' })} variant="outlined">
        <Descriptions
          column={{ xs: 1, sm: 2, lg: 4 }}
          items={[
            {
              key: 'goVersion',
              label: intl.formatMessage({ id: 'monitor.runtime.goVersion' }),
              children: snapshot.goVersion,
            },
            {
              key: 'uptime',
              label: intl.formatMessage({ id: 'monitor.runtime.uptime' }),
              children: formatUptime(snapshot.uptime, locale),
            },
            {
              key: 'goroutines',
              label: intl.formatMessage({ id: 'monitor.runtime.goroutines' }),
              children: new Intl.NumberFormat(locale).format(snapshot.runtime.goroutines),
            },
            {
              key: 'heap',
              label: intl.formatMessage({ id: 'monitor.runtime.heap' }),
              children: formatBytes(snapshot.runtime.heapAlloc, locale),
            },
            {
              key: 'gc',
              label: intl.formatMessage({ id: 'monitor.runtime.gc' }),
              children: new Intl.NumberFormat(locale).format(snapshot.runtime.numGC),
            },
            {
              key: 'collectedAt',
              label: intl.formatMessage({ id: 'monitor.runtime.collectedAt' }),
              children: collectedAt,
            },
            {
              key: 'instance',
              label: intl.formatMessage({ id: 'monitor.runtime.instance' }),
              children: <Tag>{snapshot.instanceId}</Tag>,
              span: { xs: 1, sm: 2 },
            },
          ]}
        />
      </ProCard>
    </Space>
  );
}

export default function MonitorOverview() {
  const intl = useIntl();
  const monitor = useMonitorSnapshot();
  const status = getRequestStatus(monitor.error);

  if (monitor.isPending && !monitor.data) return <PageLoading rows={8} />;
  if (!monitor.data && status === 403) {
    return <PageForbidden message={intl.formatMessage({ id: 'monitor.forbidden' })} />;
  }
  if (!monitor.data && status === 503) {
    return (
      <Alert
        action={
          <Button
            icon={<ReloadOutlined />}
            loading={monitor.isFetching}
            size="small"
            onClick={() => void monitor.refetch()}
          >
            {intl.formatMessage({ id: 'actions.retry' })}
          </Button>
        }
        description={intl.formatMessage({ id: 'monitor.warming.description' })}
        title={intl.formatMessage({ id: 'monitor.warming.title' })}
        showIcon
        type="info"
      />
    );
  }
  if (!monitor.data && monitor.isError) {
    return (
      <PageError
        message={getRequestErrorMessage(monitor.error)}
        onRetry={() => void monitor.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
        title={intl.formatMessage({ id: 'states.loadError' })}
      />
    );
  }
  if (!monitor.data) {
    return <PageEmpty description={intl.formatMessage({ id: 'monitor.empty' })} />;
  }

  return (
    <Space orientation="vertical" size="middle" className="w-full">
      <div className="flex justify-end">
        <Button
          icon={<ReloadOutlined />}
          loading={monitor.isFetching}
          onClick={() => void monitor.refetch()}
        >
          {intl.formatMessage({ id: 'actions.refresh' })}
        </Button>
      </div>
      {monitor.isError ? (
        <Alert
          description={intl.formatMessage({ id: 'monitor.refreshFailed.description' })}
          title={intl.formatMessage({ id: 'monitor.refreshFailed.title' })}
          showIcon
          type="warning"
        />
      ) : null}
      {monitor.data.stale ? (
        <Alert
          description={intl.formatMessage({ id: 'monitor.stale.description' })}
          title={intl.formatMessage({ id: 'monitor.stale.title' })}
          showIcon
          type="warning"
        />
      ) : null}
      <MonitorContent snapshot={monitor.data} />
    </Space>
  );
}
