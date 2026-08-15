export const MONITOR_HISTORY_LIMIT = 120;

export interface MonitorHistoryPoint {
  timestamp: number;
  cpuUsage: number;
  memoryUsagePercent: number;
}

export interface MonitorRuntime {
  goroutines: number;
  heapAlloc: number;
  numGC: number;
}

export interface MonitorSnapshot {
  cpuPhysicalCore: number;
  cpuLogicalCore: number;
  cpuUsage: number;
  memoryTotal: number;
  memoryUsage: number;
  memoryUsagePercent: number;
  diskTotal: number;
  diskUsage: number;
  diskUsagePercent: number;
  runtime: MonitorRuntime;
  goVersion: string;
  startTime: number;
  uptime: number;
  collectedAt: number;
  sampleIntervalMs: number;
  stale: boolean;
  instanceId: string;
  history: MonitorHistoryPoint[];
}

export class MonitorContractError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'MonitorContractError';
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function numberField(
  value: Record<string, unknown>,
  key: string,
  options: { integer?: boolean; min?: number; max?: number } = {},
): number {
  const candidate = value[key];
  if (
    typeof candidate !== 'number' ||
    !Number.isFinite(candidate) ||
    (options.integer && !Number.isSafeInteger(candidate)) ||
    (options.min !== undefined && candidate < options.min) ||
    (options.max !== undefined && candidate > options.max)
  ) {
    throw new MonitorContractError(`Monitor field ${key} is invalid`);
  }
  return candidate;
}

function stringField(value: Record<string, unknown>, key: string): string {
  const candidate = value[key];
  if (typeof candidate !== 'string' || !candidate.trim()) {
    throw new MonitorContractError(`Monitor field ${key} is invalid`);
  }
  return candidate.trim();
}

function percentField(value: Record<string, unknown>, key: string): number {
  return numberField(value, key, { min: 0, max: 100 });
}

function parseRuntime(value: unknown): MonitorRuntime {
  if (!isRecord(value)) throw new MonitorContractError('Monitor runtime is invalid');
  return {
    goroutines: numberField(value, 'goroutines', { integer: true, min: 0 }),
    heapAlloc: numberField(value, 'heapAlloc', { integer: true, min: 0 }),
    numGC: numberField(value, 'numGC', { integer: true, min: 0 }),
  };
}

function parseHistory(value: unknown): MonitorHistoryPoint[] {
  if (!Array.isArray(value)) throw new MonitorContractError('Monitor history is invalid');
  const byTimestamp = new Map<number, MonitorHistoryPoint>();
  for (const candidate of value) {
    if (!isRecord(candidate)) throw new MonitorContractError('Monitor history point is invalid');
    const point = {
      timestamp: numberField(candidate, 'timestamp', { integer: true, min: 0 }),
      cpuUsage: percentField(candidate, 'cpuUsage'),
      memoryUsagePercent: percentField(candidate, 'memoryUsagePercent'),
    };
    byTimestamp.set(point.timestamp, point);
  }
  return [...byTimestamp.values()]
    .sort((left, right) => left.timestamp - right.timestamp)
    .slice(-MONITOR_HISTORY_LIMIT);
}

export function parseMonitorSnapshot(value: unknown): MonitorSnapshot {
  if (!isRecord(value)) throw new MonitorContractError('Monitor response must be an object');
  if (typeof value.stale !== 'boolean') {
    throw new MonitorContractError('Monitor field stale is invalid');
  }
  return {
    cpuPhysicalCore: numberField(value, 'cpuPhysicalCore', { integer: true, min: 0 }),
    cpuLogicalCore: numberField(value, 'cpuLogicalCore', { integer: true, min: 0 }),
    cpuUsage: percentField(value, 'cpuUsage'),
    memoryTotal: numberField(value, 'memoryTotal', { integer: true, min: 0 }),
    memoryUsage: numberField(value, 'memoryUsage', { integer: true, min: 0 }),
    memoryUsagePercent: percentField(value, 'memoryUsagePercent'),
    diskTotal: numberField(value, 'diskTotal', { min: 0 }),
    diskUsage: numberField(value, 'diskUsage', { min: 0 }),
    diskUsagePercent: percentField(value, 'diskUsagePercent'),
    runtime: parseRuntime(value.runtime),
    goVersion: stringField(value, 'goVersion'),
    startTime: numberField(value, 'startTime', { integer: true, min: 0 }),
    uptime: numberField(value, 'uptime', { integer: true, min: 0 }),
    collectedAt: numberField(value, 'collectedAt', { integer: true, min: 0 }),
    sampleIntervalMs: numberField(value, 'sampleIntervalMs', {
      integer: true,
      min: 1_000,
      max: 300_000,
    }),
    stale: value.stale,
    instanceId: stringField(value, 'instanceId'),
    history: parseHistory(value.history),
  };
}
