import { describe, expect, it, vi } from 'vitest';
import { createMonitorAPI } from './api';

const response = {
  cpuPhysicalCore: 2,
  cpuLogicalCore: 4,
  cpuUsage: 5,
  memoryTotal: 100,
  memoryUsage: 50,
  memoryUsagePercent: 50,
  diskTotal: 200,
  diskUsage: 100,
  diskUsagePercent: 50,
  runtime: { goroutines: 10, heapAlloc: 1024, numGC: 1 },
  goVersion: 'go1.25',
  startTime: 1,
  uptime: 2,
  collectedAt: 3,
  sampleIntervalMs: 5_000,
  stale: false,
  instanceId: 'instance-a',
  history: [],
};

describe('monitor API', () => {
  it('uses the protected relative endpoint with a bounded history request', async () => {
    const client = vi.fn(async () => response);
    const api = createMonitorAPI(client);

    await expect(api.loadSnapshot()).resolves.toMatchObject({ instanceId: 'instance-a' });
    expect(client).toHaveBeenCalledWith('/monitor', {
      method: 'GET',
      params: { historyLimit: 120 },
      skipErrorHandler: true,
    });
  });
});
