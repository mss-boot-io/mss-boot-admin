import { useCallback, useEffect, useRef, useState } from 'react';
import { getMonitor } from '@/services/admin/monitor';

export interface MonitorRuntimeInfo {
  goroutines?: number;
  heapAlloc?: number;
  numGC?: number;
}

export interface MonitorData extends API.MonitorResponse {
  runtime?: MonitorRuntimeInfo;
  uptime?: number;
}

export interface MonitorHistoryData {
  time: string;
  cpu: number;
  memory: number;
}

export interface UseMonitorDataOptions {
  pollInterval?: number;
  maxHistoryItems?: number;
  onError?: (error: Error) => void;
  /** Continue polling in a background tab. Defaults to false to protect production services. */
  pollWhenHidden?: boolean;
}

export const useMonitorData = (options: UseMonitorDataOptions = {}) => {
  const { pollInterval = 60000, maxHistoryItems = 20, onError, pollWhenHidden = false } = options;

  const [monitorData, setMonitorData] = useState<MonitorData | null>(null);
  const [historyData, setHistoryData] = useState<MonitorHistoryData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const timeoutIdRef = useRef<ReturnType<typeof setTimeout>>();
  const inFlightRef = useRef(false);
  const mountedRef = useRef(true);
  const onErrorRef = useRef(onError);

  useEffect(() => {
    onErrorRef.current = onError;
  }, [onError]);

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
    try {
      const res = await getMonitor();
      if (!mountedRef.current) {
        return;
      }

      setMonitorData(res);
      setError(null);

      setHistoryData((prev) => {
        const now = new Date().toLocaleTimeString();
        const newItem = {
          time: now,
          cpu: res.cpuUsage || 0,
          memory: res.memoryUsagePercent || 0,
        };
        return [...prev, newItem].slice(-maxHistoryItems);
      });
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to fetch monitor data');
      if (mountedRef.current) {
        setError(error);
      }
      onErrorRef.current?.(error);
    } finally {
      if (mountedRef.current) {
        setLoading(false);
      }
      inFlightRef.current = false;
    }
  }, [maxHistoryItems]);

  useEffect(() => {
    let cancelled = false;

    const clearScheduledPoll = () => {
      if (timeoutIdRef.current) {
        clearTimeout(timeoutIdRef.current);
        timeoutIdRef.current = undefined;
      }
    };

    const shouldPause = () => !pollWhenHidden && document.visibilityState === 'hidden';
    let runPoll: () => Promise<void>;

    const schedulePoll = () => {
      if (!cancelled && pollInterval > 0) {
        timeoutIdRef.current = setTimeout(() => {
          void runPoll();
        }, pollInterval);
      }
    };

    runPoll = async () => {
      if (cancelled || inFlightRef.current) {
        return;
      }

      if (shouldPause()) {
        schedulePoll();
        return;
      }

      await fetchData();
      schedulePoll();
    };

    const onVisibilityChange = () => {
      if (document.visibilityState !== 'visible' || inFlightRef.current) {
        return;
      }

      clearScheduledPoll();
      void runPoll();
    };

    document.addEventListener('visibilitychange', onVisibilityChange);
    void runPoll();

    return () => {
      cancelled = true;
      clearScheduledPoll();
      document.removeEventListener('visibilitychange', onVisibilityChange);
    };
  }, [fetchData, pollInterval, pollWhenHidden]);

  const refresh = useCallback(() => {
    setLoading(true);
    return fetchData();
  }, [fetchData]);

  return {
    monitorData,
    historyData,
    loading,
    error,
    refresh,
  };
};

export default useMonitorData;
