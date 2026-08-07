import { useCallback, useEffect, useRef, useState } from 'react';
import { history } from '@umijs/max';
import { getMonitor } from '@/services/admin/monitor';
import { clearAuthStorage } from '@/utils/authStorage';

export const MONITOR_POLL_INTERVAL_MS = 5000;
export const MONITOR_HISTORY_LIMIT = 120;
export const MONITOR_MAX_RETRY_INTERVAL_MS = 60_000;
export const MONITOR_MAX_SAMPLE_INTERVAL_MS = 5 * 60_000;

const MONITOR_MIN_SERVER_DELAY_MS = 1000;

export interface MonitorRuntimeInfo {
  goroutines?: number;
  heapAlloc?: number;
  numGC?: number;
}

export interface MonitorHistorySample {
  timestamp: number;
  cpuUsage: number;
  memoryUsagePercent: number;
}

export interface MonitorData extends API.MonitorResponse {
  runtime?: MonitorRuntimeInfo;
  uptime?: number;
  collectedAt?: number;
  sampleIntervalMs?: number;
  stale?: boolean;
  instanceId?: string;
  history?: MonitorHistorySample[];
}

export interface MonitorHistoryData {
  timestamp: number;
  cpu: number;
  memory: number;
}

export interface UseMonitorDataOptions {
  pollInterval?: number;
  historyLimit?: number;
  onError?: (error: Error) => void;
}

const finiteNumber = (value: unknown): number | undefined => {
  const numberValue = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(numberValue) ? numberValue : undefined;
};

const boundedHistoryLimit = (value: number): number =>
  Math.min(MONITOR_HISTORY_LIMIT, Math.max(0, Math.trunc(value)));

const validSampleInterval = (value: unknown): number | undefined => {
  const interval = finiteNumber(value);
  if (
    interval === undefined ||
    !Number.isSafeInteger(interval) ||
    interval < MONITOR_MIN_SERVER_DELAY_MS ||
    interval > MONITOR_MAX_SAMPLE_INTERVAL_MS
  ) {
    return undefined;
  }
  return interval;
};

const retryAfterHeader = (error: unknown): unknown => {
  if (!error || typeof error !== 'object') {
    return undefined;
  }
  const candidate = error as { response?: { headers?: unknown }; headers?: unknown };
  const headers = candidate.response?.headers ?? candidate.headers;
  if (!headers || typeof headers !== 'object') {
    return undefined;
  }
  const getHeader = (headers as { get?: (name: string) => unknown }).get;
  if (typeof getHeader === 'function') {
    return getHeader.call(headers, 'Retry-After');
  }
  const entry = Object.entries(headers).find(([name]) => name.toLowerCase() === 'retry-after');
  return entry?.[1];
};

const retryAfterDelay = (error: unknown): number | undefined => {
  const value = retryAfterHeader(error);
  if (value === undefined || value === null) {
    return undefined;
  }
  const raw = String(value).trim();
  if (!raw) {
    return undefined;
  }

  const seconds = Number(raw);
  const unboundedDelay = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(raw) - Date.now();
  if (!Number.isFinite(unboundedDelay) || unboundedDelay <= 0) {
    return undefined;
  }
  return Math.min(
    MONITOR_MAX_RETRY_INTERVAL_MS,
    Math.max(MONITOR_MIN_SERVER_DELAY_MS, Math.ceil(unboundedDelay)),
  );
};

/**
 * Converts the server-owned history into chart data. The response is authoritative:
 * callers replace their complete local history with this result instead of appending
 * browser timestamps or combining samples from different instances.
 */
export const normalizeMonitorHistory = (
  history: readonly MonitorHistorySample[] | null | undefined,
  maxItems = MONITOR_HISTORY_LIMIT,
): MonitorHistoryData[] => {
  const limit = boundedHistoryLimit(maxItems);
  if (!history?.length || limit === 0) {
    return [];
  }

  const samplesByTimestamp = new Map<number, MonitorHistoryData>();
  history.forEach((sample) => {
    const timestamp = finiteNumber(sample?.timestamp);
    const cpu = finiteNumber(sample?.cpuUsage);
    const memory = finiteNumber(sample?.memoryUsagePercent);
    if (timestamp === undefined || timestamp < 0 || cpu === undefined || memory === undefined) {
      return;
    }

    // A later item wins when a response contains a duplicate timestamp.
    samplesByTimestamp.set(timestamp, { timestamp, cpu, memory });
  });

  return [...samplesByTimestamp.values()]
    .sort((left, right) => left.timestamp - right.timestamp)
    .slice(-limit);
};

const monitorErrorStatus = (error: unknown): number | undefined => {
  if (!error || typeof error !== 'object') {
    return undefined;
  }

  const candidate = error as {
    status?: unknown;
    response?: { status?: unknown };
    data?: { status?: unknown };
  };
  return finiteNumber(candidate.response?.status ?? candidate.status ?? candidate.data?.status);
};

const toError = (error: unknown): Error => {
  if (error instanceof Error) {
    return error;
  }
  if (error && typeof error === 'object' && 'message' in error) {
    return new Error(String(error.message));
  }
  return new Error('Failed to fetch monitor data');
};

export const useMonitorData = (options: UseMonitorDataOptions = {}) => {
  const {
    pollInterval = MONITOR_POLL_INTERVAL_MS,
    historyLimit = MONITOR_HISTORY_LIMIT,
    onError,
  } = options;
  const normalizedHistoryLimit = boundedHistoryLimit(historyLimit);

  const [monitorData, setMonitorData] = useState<MonitorData | null>(null);
  const [historyData, setHistoryData] = useState<MonitorHistoryData[]>([]);
  const [lastUpdated, setLastUpdated] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [serverStale, setServerStale] = useState(false);
  const [notReady, setNotReady] = useState(false);
  const [permissionDenied, setPermissionDenied] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [pollGeneration, setPollGeneration] = useState(0);
  const timeoutIdRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const inFlightRef = useRef(false);
  const mountedRef = useRef(true);
  const onErrorRef = useRef(onError);
  const terminalErrorRef = useRef(false);
  const basePollDelayRef = useRef(pollInterval);
  const nextPollDelayRef = useRef(pollInterval);

  useEffect(() => {
    onErrorRef.current = onError;
  }, [onError]);

  useEffect(() => {
    basePollDelayRef.current = pollInterval;
    nextPollDelayRef.current = pollInterval;
  }, [pollInterval]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const fetchData = useCallback(async () => {
    if (inFlightRef.current) {
      return;
    }

    inFlightRef.current = true;
    if (mountedRef.current) {
      setRefreshing(true);
    }

    try {
      const rawResponse = await getMonitor({
        params: { historyLimit: normalizedHistoryLimit },
        // Polling owns its loading/not-ready/stale UI. Suppress the global
        // request toast so a temporary 503 or refresh failure is not repeated
        // every five seconds.
        skipErrorHandler: true,
      });
      if (!mountedRef.current) {
        return;
      }

      const response = rawResponse as MonitorData;
      const nextHistory = normalizeMonitorHistory(response.history, normalizedHistoryLimit);
      const collectedAt = finiteNumber(response.collectedAt);
      const newestHistoryTimestamp = nextHistory[nextHistory.length - 1]?.timestamp;

      // Every successful response replaces the complete history. This prevents both
      // duplicate points and accidental joins across an instance restart/change.
      setMonitorData(response);
      setHistoryData(nextHistory);
      setLastUpdated(collectedAt ?? newestHistoryTimestamp ?? null);
      setServerStale(Boolean(response.stale));
      setNotReady(false);
      setPermissionDenied(false);
      setError(null);
      terminalErrorRef.current = false;
      const sampleInterval = validSampleInterval(response.sampleIntervalMs);
      basePollDelayRef.current = Math.max(pollInterval, sampleInterval ?? pollInterval);
      nextPollDelayRef.current = basePollDelayRef.current;
    } catch (caughtError) {
      const nextError = toError(caughtError);
      const status = monitorErrorStatus(caughtError);
      // A request may settle after the component has already navigated away.
      // Do not let that stale result clear the active session, redirect, or
      // notify a caller that no longer owns this polling lifecycle.
      if (!mountedRef.current) {
        return;
      }

      const terminalError = status === 401 || status === 403;
      terminalErrorRef.current = terminalError;
      if (status === 401) {
        clearAuthStorage();
        history.push('/user/login');
      }
      setError(nextError);
      setNotReady(status === 503);
      setPermissionDenied(status === 403);
      if (terminalError) {
        setMonitorData(null);
        setHistoryData([]);
        setLastUpdated(null);
        setServerStale(false);
      }
      if (status === 503) {
        // Reaching the backend means connectivity recovered. Respect its
        // readiness hint without allowing malformed values to create a zero
        // delay or an unbounded timer.
        nextPollDelayRef.current = Math.max(
          basePollDelayRef.current,
          retryAfterDelay(caughtError) ?? pollInterval,
        );
      } else if (!terminalError && pollInterval > 0) {
        nextPollDelayRef.current = Math.max(
          basePollDelayRef.current,
          Math.min(
            MONITOR_MAX_RETRY_INTERVAL_MS,
            Math.max(pollInterval, nextPollDelayRef.current * 2),
          ),
        );
      }
      onErrorRef.current?.(nextError);
    } finally {
      if (mountedRef.current) {
        setLoading(false);
        setRefreshing(false);
      }
      inFlightRef.current = false;
    }
  }, [normalizedHistoryLimit, pollInterval]);

  useEffect(() => {
    let cancelled = false;

    const clearScheduledPoll = () => {
      if (timeoutIdRef.current !== undefined) {
        clearTimeout(timeoutIdRef.current);
        timeoutIdRef.current = undefined;
      }
    };

    const isHidden = () => document.visibilityState === 'hidden';
    let runPoll: () => Promise<void>;

    const schedulePoll = () => {
      clearScheduledPoll();
      if (!cancelled && !terminalErrorRef.current && !isHidden() && pollInterval > 0) {
        timeoutIdRef.current = setTimeout(() => {
          void runPoll();
        }, nextPollDelayRef.current);
      }
    };

    runPoll = async () => {
      if (cancelled || terminalErrorRef.current || isHidden()) {
        return;
      }

      await fetchData();
      schedulePoll();
    };

    const onVisibilityChange = () => {
      clearScheduledPoll();
      if (!terminalErrorRef.current && !isHidden()) {
        void runPoll();
      }
    };

    document.addEventListener('visibilitychange', onVisibilityChange);
    if (!isHidden()) {
      void runPoll();
    }

    return () => {
      cancelled = true;
      clearScheduledPoll();
      document.removeEventListener('visibilitychange', onVisibilityChange);
    };
  }, [fetchData, pollGeneration, pollInterval]);

  const refresh = useCallback(() => {
    terminalErrorRef.current = false;
    nextPollDelayRef.current = basePollDelayRef.current;
    setPollGeneration((generation) => generation + 1);
  }, []);

  return {
    monitorData,
    historyData,
    lastUpdated,
    loading,
    refreshing,
    stale: serverStale || Boolean(error && monitorData),
    notReady,
    permissionDenied,
    error,
    refresh,
  };
};

export default useMonitorData;
