import { fireEvent, render, screen } from '@testing-library/react';
import { App } from 'antd';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { MonitorSnapshot } from './contract';
import MonitorOverview from './MonitorOverview';

const monitorQuery = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'en-US',
    formatMessage: ({ id }: { id: string }, values?: Record<string, unknown>) =>
      values ? `${id}:${Object.values(values).join('|')}` : id,
  }),
}));

vi.mock('./query', () => ({
  useMonitorSnapshot: () => monitorQuery.current,
}));

function snapshot(overrides: Partial<MonitorSnapshot> = {}): MonitorSnapshot {
  return {
    cpuPhysicalCore: 4,
    cpuLogicalCore: 8,
    cpuUsage: 25,
    memoryTotal: 16 * 1024 ** 3,
    memoryUsage: 8 * 1024 ** 3,
    memoryUsagePercent: 50,
    diskTotal: 200,
    diskUsage: 80,
    diskUsagePercent: 40,
    runtime: { goroutines: 42, heapAlloc: 1024 ** 2, numGC: 7 },
    goVersion: 'go1.25',
    startTime: 1_700_000_000,
    uptime: 3_900,
    collectedAt: 1_700_000_000_000,
    sampleIntervalMs: 5_000,
    stale: false,
    instanceId: 'instance-a',
    history: [
      { timestamp: 1_000, cpuUsage: 10, memoryUsagePercent: 30 },
      { timestamp: 2_000, cpuUsage: 25, memoryUsagePercent: 50 },
    ],
    ...overrides,
  };
}

function renderOverview() {
  return render(
    <App>
      <MonitorOverview />
    </App>,
  );
}

describe('monitor overview states', () => {
  beforeEach(() => {
    monitorQuery.current = {
      data: undefined,
      error: null,
      isError: false,
      isFetching: false,
      isPending: false,
      refetch: vi.fn(),
    };
  });

  it('renders an initial loading state before the first authoritative sample arrives', () => {
    monitorQuery.current = { ...monitorQuery.current, isPending: true };

    renderOverview();

    expect(screen.getByRole('status')).toBeTruthy();
  });

  it('fails closed when the monitor endpoint denies permission', () => {
    monitorQuery.current = {
      ...monitorQuery.current,
      error: { response: { status: 403 } },
      isError: true,
    };

    renderOverview();

    expect(screen.getByText('403')).toBeTruthy();
    expect(screen.getByText('monitor.forbidden')).toBeTruthy();
  });

  it('shows the warm-up state and allows an explicit retry for 503', () => {
    const refetch = vi.fn();
    monitorQuery.current = {
      ...monitorQuery.current,
      error: { response: { status: 503 } },
      isError: true,
      refetch,
    };

    renderOverview();
    fireEvent.click(screen.getByRole('button', { name: /actions\.retry/ }));

    expect(screen.getByText('monitor.warming.title')).toBeTruthy();
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it('shows a retryable error without inventing an empty snapshot', () => {
    const refetch = vi.fn();
    monitorQuery.current = {
      ...monitorQuery.current,
      error: new Error('monitor offline'),
      isError: true,
      refetch,
    };

    renderOverview();
    fireEvent.click(screen.getByRole('button', { name: /actions\.retry/ }));

    expect(screen.getByText('monitor offline')).toBeTruthy();
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it('retains the last successful snapshot when a background refresh fails', () => {
    monitorQuery.current = {
      ...monitorQuery.current,
      data: snapshot(),
      error: new Error('temporary'),
      isError: true,
    };

    renderOverview();

    expect(screen.getByText('monitor.refreshFailed.title')).toBeTruthy();
    expect(screen.getByText('instance-a')).toBeTruthy();
    expect(screen.getByRole('img', { name: 'monitor.cpu.trendLabel' })).toBeTruthy();
    expect(screen.getByRole('img', { name: 'monitor.memory.trendLabel' })).toBeTruthy();
  });

  it('marks server-provided last-good data as stale without hiding it', () => {
    monitorQuery.current = {
      ...monitorQuery.current,
      data: snapshot({ stale: true, history: [] }),
    };

    renderOverview();

    expect(screen.getByText('monitor.stale.title')).toBeTruthy();
    expect(screen.getAllByText('monitor.history.empty')).toHaveLength(2);
    expect(screen.getByText('instance-a')).toBeTruthy();
  });
});
