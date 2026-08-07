import { render, screen } from '@testing-library/react';
import MonitorTrend, { createMonitorTrendConfig, isDarkMonitorBackground } from './index';

const React = require('react');

jest.mock('@/hooks/useResponsive', () => ({
  useResponsive: () => ({ isMobile: false, isTablet: false, isDesktop: true, screens: {} }),
}));

const data = [{ timestamp: 1000, cpu: 25, memory: 50 }];

describe('MonitorTrend config', () => {
  it('uses the classic theme and light tokens in light mode', () => {
    const config = createMonitorTrendConfig({
      data,
      metric: 'cpu',
      metricLabel: 'CPU Usage',
      dark: false,
      mobile: false,
      palette: {
        colorPrimary: '#1677ff',
        colorTextSecondary: '#666666',
        colorBorderSecondary: '#eeeeee',
        colorBgContainer: '#ffffff',
      },
    }) as any;

    expect(config.xField).toBe('timestamp');
    expect(config.yField).toBe('cpu');
    expect(config.theme).toEqual({ type: 'classic' });
    expect(config.axis.x.labelFill).toBe('#666666');
    expect(config.axis.y.gridStroke).toBe('#eeeeee');
    expect(config.style.fill).toBe('#1677ff');
    expect(config.line.style.stroke).toBe('#1677ff');
    expect(config.viewStyle).toEqual({ viewFill: 'transparent', plotFill: 'transparent' });
    expect(config.tooltip).toBeDefined();
  });

  it('uses classicDark and dark tokens without introducing a light chart background', () => {
    const config = createMonitorTrendConfig({
      data,
      metric: 'memory',
      metricLabel: 'Memory Usage',
      dark: true,
      mobile: true,
      palette: {
        colorPrimary: '#1668dc',
        colorTextSecondary: '#8c8c8c',
        colorBorderSecondary: '#303030',
        colorBgContainer: '#141414',
      },
    }) as any;

    expect(config.theme).toEqual({ type: 'classicDark' });
    expect(config.axis.x.labelFill).toBe('#8c8c8c');
    expect(config.axis.y.gridStroke).toBe('#303030');
    expect(config.style.fill).toBe('#1668dc');
    expect(config.line.style.stroke).toBe('#1668dc');
    expect(config.viewStyle.viewFill).toBe('transparent');
    expect(config.viewStyle.plotFill).toBe('transparent');
    expect(config.height).toBe(180);
    expect(config.tooltip).toBeDefined();
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
});
