import type { AreaConfig } from '@ant-design/charts';
import { Empty, Skeleton, theme } from 'antd';
import React, { useEffect, useMemo, useRef, useState } from 'react';
import type { MonitorHistoryData } from '@/hooks/useMonitorData';
import { useResponsive } from '@/hooks/useResponsive';

const LazyArea = React.lazy(async () => {
  const { Area } = await import('@ant-design/charts');
  return { default: Area };
});

export type MonitorTrendMetric = 'cpu' | 'memory';

export interface MonitorTrendPalette {
  colorPrimary: string;
  colorTextSecondary: string;
  colorBorderSecondary: string;
  colorBgContainer: string;
}

export interface CreateMonitorTrendConfigOptions {
  data: MonitorHistoryData[];
  metric: MonitorTrendMetric;
  metricLabel: string;
  palette: MonitorTrendPalette;
  dark: boolean;
  mobile: boolean;
}

export interface MonitorTrendProps {
  data: MonitorHistoryData[];
  metric: MonitorTrendMetric;
  metricLabel: string;
  emptyDescription: React.ReactNode;
}

export const formatMonitorTime = (value: unknown): string => {
  const timestamp = value instanceof Date ? value.getTime() : Number(value);
  if (!Number.isFinite(timestamp)) {
    return '';
  }

  return new Intl.DateTimeFormat(undefined, {
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

export const createMonitorTrendConfig = ({
  data,
  metric,
  metricLabel,
  palette,
  dark,
  mobile,
}: CreateMonitorTrendConfigOptions): AreaConfig =>
  ({
    data,
    xField: 'timestamp',
    yField: metric,
    autoFit: true,
    height: mobile ? 180 : 200,
    animate: false,
    theme: { type: dark ? 'classicDark' : 'classic' },
    viewStyle: {
      viewFill: 'transparent',
      plotFill: 'transparent',
    },
    scale: {
      x: { type: 'time' },
      y: { domain: [0, 100], nice: true },
    },
    axis: {
      x: {
        tickCount: mobile ? 3 : 6,
        labelFormatter: formatMonitorTime,
        labelFill: palette.colorTextSecondary,
        lineStroke: palette.colorBorderSecondary,
        tickStroke: palette.colorBorderSecondary,
        grid: false,
      },
      y: {
        tickCount: 5,
        labelFormatter: (value: unknown) => `${Number(value)}%`,
        labelFill: palette.colorTextSecondary,
        lineStroke: palette.colorBorderSecondary,
        tickStroke: palette.colorBorderSecondary,
        gridStroke: palette.colorBorderSecondary,
        gridStrokeOpacity: dark ? 0.35 : 0.65,
      },
    },
    style: {
      fill: palette.colorPrimary,
      fillOpacity: dark ? 0.28 : 0.2,
    },
    line: {
      style: {
        stroke: palette.colorPrimary,
        lineWidth: 2,
      },
    },
    tooltip: {
      title: (datum: MonitorHistoryData) => formatMonitorTime(datum.timestamp),
      items: [
        {
          field: metric,
          name: metricLabel,
          valueFormatter: (value: unknown) => `${Number(value).toFixed(2)}%`,
        },
      ],
    },
  } as AreaConfig);

const DeferredArea: React.FC<{ config: AreaConfig }> = ({ config }) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);
  const height = typeof config.height === 'number' ? config.height : 200;

  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return undefined;
    }

    if (!('IntersectionObserver' in window)) {
      setVisible(true);
      return undefined;
    }

    const observer = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting) {
        setVisible(true);
        observer.disconnect();
      }
    });

    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  return (
    <div ref={containerRef} style={{ minHeight: height }}>
      {visible ? (
        <React.Suspense fallback={<Skeleton active paragraph={{ rows: 5 }} />}>
          <LazyArea {...config} />
        </React.Suspense>
      ) : (
        <Skeleton active paragraph={{ rows: 5 }} />
      )}
    </div>
  );
};

export const MonitorTrend: React.FC<MonitorTrendProps> = ({
  data,
  metric,
  metricLabel,
  emptyDescription,
}) => {
  const { token } = theme.useToken();
  const { isMobile } = useResponsive();
  const dark = isDarkMonitorBackground(token.colorBgContainer);
  const config = useMemo(
    () =>
      createMonitorTrendConfig({
        data,
        metric,
        metricLabel,
        dark,
        mobile: isMobile,
        palette: {
          colorPrimary: token.colorPrimary,
          colorTextSecondary: token.colorTextSecondary,
          colorBorderSecondary: token.colorBorderSecondary,
          colorBgContainer: token.colorBgContainer,
        },
      }),
    [data, dark, isMobile, metric, metricLabel, token],
  );

  if (data.length === 0) {
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

  return <DeferredArea config={config} />;
};

export default MonitorTrend;
