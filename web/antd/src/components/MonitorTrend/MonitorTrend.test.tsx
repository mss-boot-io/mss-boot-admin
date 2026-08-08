import { render, screen } from '@testing-library/react';
import MonitorTrend, { createMonitorTrendModel, isDarkMonitorBackground } from './index';

const React = require('react');

jest.mock('@/hooks/useResponsive', () => ({
  useResponsive: () => ({ isMobile: false, isTablet: false, isDesktop: true, screens: {} }),
}));

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: (
      _descriptor: unknown,
      values: { metric: string; count: number; value: string; time: string },
    ) => `${values.metric}: ${values.count} samples, latest ${values.value}% at ${values.time}`,
  }),
}));

const data = [
  { timestamp: 1000, cpu: 25, memory: 50 },
  { timestamp: 2000, cpu: 75, memory: 60 },
];

describe('MonitorTrend model', () => {
  it('creates bounded SVG paths and responsive ticks without a chart runtime', () => {
    const model = createMonitorTrendModel({
      data,
      metric: 'cpu',
      dark: false,
      mobile: false,
    });

    expect(model.points).toHaveLength(2);
    expect(model.points.map((point) => point.value)).toEqual([25, 75]);
    expect(model.linePath).toMatch(/^M /);
    expect(model.areaPath).toMatch(/ Z$/);
    expect(model.yTicks.map((tick) => tick.label)).toEqual(['0%', '25%', '50%', '75%', '100%']);
    expect(model.fillOpacity).toBe(0.2);
    expect(model.height).toBe(200);
  });

  it('clamps invalid percentages and adapts height for dark mobile charts', () => {
    const model = createMonitorTrendModel({
      data: [
        { timestamp: 1000, cpu: -20, memory: 50 },
        { timestamp: 2000, cpu: 120, memory: 60 },
      ],
      metric: 'cpu',
      dark: true,
      mobile: true,
    });

    expect(model.points.map((point) => point.value)).toEqual([0, 100]);
    expect(model.height).toBe(180);
    expect(model.fillOpacity).toBe(0.28);
  });

  it('preserves irregular sampling gaps on the time axis', () => {
    const model = createMonitorTrendModel({
      data: [
        { timestamp: 4000, cpu: 30, memory: 30 },
        { timestamp: 1000, cpu: 10, memory: 10 },
        { timestamp: 2000, cpu: 20, memory: 20 },
      ],
      metric: 'cpu',
      dark: false,
      mobile: false,
    });

    expect(model.points.map((point) => point.timestamp)).toEqual([1000, 2000, 4000]);
    const firstGap = model.points[1].x - model.points[0].x;
    const secondGap = model.points[2].x - model.points[1].x;
    expect(secondGap).toBeCloseTo(firstGap * 2);
  });

  it('detects the effective Ant Design container theme', () => {
    expect(isDarkMonitorBackground('#141414')).toBe(true);
    expect(isDarkMonitorBackground('rgb(255, 255, 255)')).toBe(false);
  });
});

describe('MonitorTrend states', () => {
  it('renders a meaningful empty state before history is available', () => {
    render(
      <MonitorTrend
        data={[]}
        metric="cpu"
        metricLabel="CPU Usage"
        emptyDescription="Waiting for samples"
      />,
    );

    expect(screen.getByText('Waiting for samples')).toBeTruthy();
    expect(screen.getByTestId('monitor-trend-cpu-empty')).toBeTruthy();
  });

  it('renders an accessible lightweight chart when samples exist', () => {
    render(
      <MonitorTrend data={data} metric="cpu" metricLabel="CPU Usage" emptyDescription="Empty" />,
    );

    expect(screen.getByRole('img', { name: /CPU Usage: 2 samples, latest 75.00%/ })).toBeTruthy();
    expect(screen.getByTestId('monitor-trend-chart').querySelectorAll('path')).toHaveLength(2);
  });
});
