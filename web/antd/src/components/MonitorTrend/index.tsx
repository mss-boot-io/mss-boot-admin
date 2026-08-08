import { Empty, theme } from 'antd';
import React, { useMemo } from 'react';
import { useIntl } from '@umijs/max';
import type { MonitorHistoryData } from '@/hooks/useMonitorData';
import { useResponsive } from '@/hooks/useResponsive';

export type MonitorTrendMetric = 'cpu' | 'memory';

export interface MonitorTrendPalette {
  colorPrimary: string;
  colorTextSecondary: string;
  colorBorderSecondary: string;
  colorBgContainer: string;
}

export interface CreateMonitorTrendModelOptions {
  data: MonitorHistoryData[];
  metric: MonitorTrendMetric;
  dark: boolean;
  mobile: boolean;
}

export interface MonitorTrendPoint {
  timestamp: number;
  value: number;
  x: number;
  y: number;
}

export interface MonitorTrendModel {
  areaPath: string;
  height: number;
  linePath: string;
  points: MonitorTrendPoint[];
  width: number;
  xTicks: MonitorTrendPoint[];
  yTicks: Array<{ label: string; value: number; y: number }>;
  plot: { left: number; right: number; top: number; bottom: number };
  fillOpacity: number;
}

export interface MonitorTrendProps {
  data: MonitorHistoryData[];
  metric: MonitorTrendMetric;
  metricLabel: string;
  emptyDescription: React.ReactNode;
}

const CHART_WIDTH = 800;
const CHART_PADDING = { left: 48, right: 12, top: 12, bottom: 30 } as const;

export const formatMonitorTime = (value: unknown, locale?: string): string => {
  const timestamp = value instanceof Date ? value.getTime() : Number(value);
  if (!Number.isFinite(timestamp)) {
    return '';
  }

  return new Intl.DateTimeFormat(locale || undefined, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(timestamp);
};

export const isDarkMonitorBackground = (color: string): boolean => {
  const value = color.trim();
  const shortHex = /^#([\da-f])([\da-f])([\da-f])$/i.exec(value);
  const longHex = /^#([\da-f]{2})([\da-f]{2})([\da-f]{2})/i.exec(value);
  const rgb = /^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i.exec(value);

  const channels = shortHex
    ? shortHex.slice(1, 4).map((channel) => Number.parseInt(`${channel}${channel}`, 16))
    : longHex
    ? longHex.slice(1, 4).map((channel) => Number.parseInt(channel, 16))
    : rgb
    ? rgb.slice(1, 4).map(Number)
    : undefined;

  if (!channels) {
    return false;
  }

  const [red, green, blue] = channels;
  return (red * 299 + green * 587 + blue * 114) / 1000 < 128;
};

const selectTicks = <T,>(values: T[], count: number): T[] => {
  if (values.length <= count) {
    return values;
  }

  const selected = new Set<number>();
  for (let index = 0; index < count; index += 1) {
    selected.add(Math.round((index * (values.length - 1)) / (count - 1)));
  }
  return Array.from(selected).map((index) => values[index]);
};

export const createMonitorTrendModel = ({
  data,
  metric,
  dark,
  mobile,
}: CreateMonitorTrendModelOptions): MonitorTrendModel => {
  const height = mobile ? 180 : 200;
  const width = mobile ? 360 : CHART_WIDTH;
  const plot = {
    left: CHART_PADDING.left,
    right: width - CHART_PADDING.right,
    top: CHART_PADDING.top,
    bottom: height - CHART_PADDING.bottom,
  };
  const plotWidth = plot.right - plot.left;
  const plotHeight = plot.bottom - plot.top;
  const normalized = data
    .map((sample) => ({ timestamp: Number(sample.timestamp), value: Number(sample[metric]) }))
    .filter((sample) => Number.isFinite(sample.timestamp) && Number.isFinite(sample.value))
    .sort((left, right) => left.timestamp - right.timestamp);
  const firstTimestamp = normalized[0]?.timestamp;
  const lastTimestamp = normalized[normalized.length - 1]?.timestamp;
  const timeRange =
    firstTimestamp === undefined || lastTimestamp === undefined
      ? 0
      : lastTimestamp - firstTimestamp;
  const points = normalized.map((sample) => {
    const ratio = timeRange <= 0 ? 0.5 : (sample.timestamp - firstTimestamp!) / timeRange;
    const value = Math.min(100, Math.max(0, sample.value));
    return {
      ...sample,
      value,
      x: plot.left + ratio * plotWidth,
      y: plot.top + (1 - value / 100) * plotHeight,
    };
  });
  const linePath = points
    .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x.toFixed(2)} ${point.y.toFixed(2)}`)
    .join(' ');
  const areaPath = points.length
    ? `M ${points[0].x.toFixed(2)} ${plot.bottom} ${points
        .map((point) => `L ${point.x.toFixed(2)} ${point.y.toFixed(2)}`)
        .join(' ')} L ${points[points.length - 1].x.toFixed(2)} ${plot.bottom} Z`
    : '';

  return {
    areaPath,
    height,
    linePath,
    points,
    width,
    xTicks: selectTicks(points, mobile ? 3 : 6),
    yTicks: [0, 25, 50, 75, 100].map((value) => ({
      label: `${value}%`,
      value,
      y: plot.top + (1 - value / 100) * plotHeight,
    })),
    plot,
    fillOpacity: dark ? 0.28 : 0.2,
  };
};

const TrendSvg: React.FC<{
  accessibleLabel: string;
  model: MonitorTrendModel;
  metricLabel: string;
  palette: MonitorTrendPalette;
  locale?: string;
}> = ({ accessibleLabel, model, metricLabel, palette, locale }) => {
  return (
    <figure style={{ margin: 0 }}>
      <svg
        role="img"
        aria-label={accessibleLabel}
        data-testid="monitor-trend-chart"
        viewBox={`0 0 ${model.width} ${model.height}`}
        width={model.width}
        height={model.height}
        style={{ display: 'block', width: '100%', height: 'auto' }}
      >
        {model.yTicks.map((tick) => (
          <g key={tick.value}>
            <line
              x1={model.plot.left}
              x2={model.plot.right}
              y1={tick.y}
              y2={tick.y}
              stroke={palette.colorBorderSecondary}
              strokeOpacity={0.65}
              vectorEffect="non-scaling-stroke"
            />
            <text
              x={model.plot.left - 8}
              y={tick.y + 4}
              textAnchor="end"
              fill={palette.colorTextSecondary}
              fontSize={11}
            >
              {tick.label}
            </text>
          </g>
        ))}
        <path d={model.areaPath} fill={palette.colorPrimary} fillOpacity={model.fillOpacity} />
        <path
          d={model.linePath}
          fill="none"
          stroke={palette.colorPrimary}
          strokeWidth={2}
          vectorEffect="non-scaling-stroke"
        />
        {model.xTicks.map((tick, tickIndex) => (
          <text
            key={tick.timestamp}
            x={tick.x}
            y={model.height - 8}
            textAnchor={tickIndex === model.xTicks.length - 1 ? 'end' : 'middle'}
            fill={palette.colorTextSecondary}
            fontSize={11}
          >
            {formatMonitorTime(tick.timestamp, locale)}
          </text>
        ))}
        {model.points.map((point) => (
          <circle
            key={`${point.timestamp}-${point.x}`}
            aria-hidden="true"
            cx={point.x}
            cy={point.y}
            r={6}
            fill="transparent"
            stroke="transparent"
          >
            <title>{`${formatMonitorTime(point.timestamp, locale)} · ${metricLabel}: ${point.value.toFixed(
              2,
            )}%`}</title>
          </circle>
        ))}
      </svg>
    </figure>
  );
};

export const MonitorTrend: React.FC<MonitorTrendProps> = ({
  data,
  metric,
  metricLabel,
  emptyDescription,
}) => {
  const intl = useIntl();
  const { token } = theme.useToken();
  const { isMobile } = useResponsive();
  const palette = useMemo<MonitorTrendPalette>(
    () => ({
      colorPrimary: token.colorPrimary,
      colorTextSecondary: token.colorTextSecondary,
      colorBorderSecondary: token.colorBorderSecondary,
      colorBgContainer: token.colorBgContainer,
    }),
    [token.colorBgContainer, token.colorBorderSecondary, token.colorPrimary, token.colorTextSecondary],
  );
  const model = useMemo(
    () =>
      createMonitorTrendModel({
        data,
        metric,
        dark: isDarkMonitorBackground(token.colorBgContainer),
        mobile: isMobile,
      }),
    [data, isMobile, metric, palette, token.colorBgContainer],
  );

  if (model.points.length === 0) {
    return (
      <div
        aria-live="polite"
        data-testid={`monitor-trend-${metric}-empty`}
        style={{ alignItems: 'center', display: 'flex', justifyContent: 'center', minHeight: 200 }}
      >
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyDescription} />
      </div>
    );
  }

  const latest = model.points[model.points.length - 1];
  const accessibleLabel = intl.formatMessage(
    {
      id: 'pages.monitor.trend.ariaLabel',
      defaultMessage: '{metric}: {count} samples, latest {value}% at {time}',
    },
    {
      metric: metricLabel,
      count: model.points.length,
      value: latest.value.toFixed(2),
      time: formatMonitorTime(latest.timestamp, intl.locale),
    },
  );

  return (
    <TrendSvg
      accessibleLabel={accessibleLabel}
      model={model}
      metricLabel={metricLabel}
      palette={palette}
      locale={intl.locale}
    />
  );
};

export default MonitorTrend;
