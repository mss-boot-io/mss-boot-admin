import { useEffect, useState, useRef } from 'react';
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
}

export const useMonitorData = (options: UseMonitorDataOptions = {}) => {
  const { pollInterval = 30000, maxHistoryItems = 20, onError } = options;

  const [monitorData, setMonitorData] = useState<MonitorData | null>(null);
  const [historyData, setHistoryData] = useState<MonitorHistoryData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const intervalIdRef = useRef<NodeJS.Timeout>();

  const fetchData = async () => {
    try {
      const res = await getMonitor();
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
      setError(error);
      onError?.(error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();

    intervalIdRef.current = setInterval(fetchData, pollInterval);

    return () => {
      if (intervalIdRef.current) {
        clearInterval(intervalIdRef.current);
      }
    };
  }, [pollInterval, maxHistoryItems]);

  const refresh = () => {
    setLoading(true);
    return fetchData();
  };

  return {
    monitorData,
    historyData,
    loading,
    error,
    refresh,
  };
};

export default useMonitorData;
