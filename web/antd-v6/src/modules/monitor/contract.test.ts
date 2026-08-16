import { describe, expect, it } from 'vitest';
import { MonitorContractError, parseMonitorSnapshot } from './contract';

function snapshot() {
  return {
    cpuPhysicalCore: 4,
    cpuLogicalCore: 8,
    cpuUsage: 22.5,
    memoryTotal: 16 * 1024 ** 3,
    memoryUsage: 8 * 1024 ** 3,
    memoryUsagePercent: 50,
    diskTotal: 100,
    diskUsage: 40,
    diskUsagePercent: 40,
    runtime: { goroutines: 42, heapAlloc: 1024, numGC: 7 },
    goVersion: 'go1.25.0',
    startTime: 1_700_000_000,
    uptime: 3600,
    collectedAt: 1_700_000_000_000,
    sampleIntervalMs: 5_000,
    stale: false,
    instanceId: 'instance-a',
    history: [
      { timestamp: 2_000, cpuUsage: 20, memoryUsagePercent: 40 },
      { timestamp: 1_000, cpuUsage: 10, memoryUsagePercent: 30 },
      { timestamp: 2_000, cpuUsage: 25, memoryUsagePercent: 45 },
    ],
  };
}

describe('monitor transport contract', () => {
  it('normalizes authoritative history ordering and duplicate timestamps', () => {
    expect(parseMonitorSnapshot(snapshot()).history).toEqual([
      { timestamp: 1_000, cpuUsage: 10, memoryUsagePercent: 30 },
      { timestamp: 2_000, cpuUsage: 25, memoryUsagePercent: 45 },
    ]);
  });

  it('rejects malformed or out-of-range metrics instead of inventing dashboard values', () => {
    expect(() => parseMonitorSnapshot({ ...snapshot(), cpuUsage: 101 })).toThrow(
      MonitorContractError,
    );
    expect(() => parseMonitorSnapshot({ ...snapshot(), stale: 'false' })).toThrow(
      /stale is invalid/,
    );
    expect(() =>
      parseMonitorSnapshot({
        ...snapshot(),
        history: [{ timestamp: 1, cpuUsage: Number.NaN, memoryUsagePercent: 10 }],
      }),
    ).toThrow(/cpuUsage is invalid/);
  });
});
