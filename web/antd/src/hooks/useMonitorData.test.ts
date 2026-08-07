import { act, renderHook } from '@testing-library/react';
import { history } from '@umijs/max';
import { getMonitor } from '@/services/admin/monitor';
import { clearTransientAuthToken, getAuthToken, setTransientAuthToken } from '@/utils/authStorage';
import {
  MONITOR_MAX_RETRY_INTERVAL_MS,
  MONITOR_MAX_SAMPLE_INTERVAL_MS,
  MONITOR_POLL_INTERVAL_MS,
  normalizeMonitorHistory,
  useMonitorData,
} from './useMonitorData';

jest.mock('@umijs/max', () => ({
  history: {
    push: jest.fn(),
  },
}));

jest.mock('@/services/admin/monitor', () => ({
  getMonitor: jest.fn(),
}));

const mockGetMonitor = getMonitor as jest.Mock;
const mockHistoryPush = history.push as jest.Mock;

const flushMicrotasks = async () => {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
};

const setDocumentVisibility = (visibilityState: DocumentVisibilityState) => {
  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    value: visibilityState,
  });
};

describe('normalizeMonitorHistory', () => {
  it('maps server timestamps, drops invalid samples, deduplicates, and sorts chronologically', () => {
    expect(
      normalizeMonitorHistory([
        { timestamp: 3000, cpuUsage: 30, memoryUsagePercent: 60 },
        { timestamp: 1000, cpuUsage: 10, memoryUsagePercent: 40 },
        { timestamp: 3000, cpuUsage: 35, memoryUsagePercent: 65 },
        { timestamp: Number.NaN, cpuUsage: 99, memoryUsagePercent: 99 },
        { timestamp: 2000, cpuUsage: 20, memoryUsagePercent: 50 },
      ]),
    ).toEqual([
      { timestamp: 1000, cpu: 10, memory: 40 },
      { timestamp: 2000, cpu: 20, memory: 50 },
      { timestamp: 3000, cpu: 35, memory: 65 },
    ]);
  });
});

describe('useMonitorData', () => {
  beforeEach(() => {
    jest.useFakeTimers();
    jest.clearAllMocks();
    clearTransientAuthToken();
    setDocumentVisibility('visible');
  });

  afterEach(() => {
    Reflect.deleteProperty(document, 'visibilityState');
    jest.useRealTimers();
  });

  it('fully replaces history for the same instance and never joins a new instance', async () => {
    mockGetMonitor
      .mockResolvedValueOnce({
        instanceId: 'instance-a',
        collectedAt: 2000,
        cpuUsage: 20,
        history: [
          { timestamp: 1000, cpuUsage: 10, memoryUsagePercent: 40 },
          { timestamp: 2000, cpuUsage: 20, memoryUsagePercent: 50 },
        ],
      })
      .mockResolvedValueOnce({
        instanceId: 'instance-a',
        collectedAt: 3000,
        cpuUsage: 30,
        history: [
          { timestamp: 3000, cpuUsage: 30, memoryUsagePercent: 60 },
          { timestamp: 3000, cpuUsage: 31, memoryUsagePercent: 61 },
        ],
      })
      .mockResolvedValueOnce({
        instanceId: 'instance-b',
        collectedAt: 4000,
        cpuUsage: 40,
        history: [{ timestamp: 4000, cpuUsage: 40, memoryUsagePercent: 70 }],
      });

    const { result } = renderHook(() => useMonitorData());

    await act(async () => {
      await flushMicrotasks();
    });
    expect(result.current.historyData.map((sample) => sample.timestamp)).toEqual([1000, 2000]);
    expect(result.current.lastUpdated).toBe(2000);
    expect(mockGetMonitor).toHaveBeenLastCalledWith({
      params: { historyLimit: 120 },
      skipErrorHandler: true,
    });

    await act(async () => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS);
      await flushMicrotasks();
    });
    expect(result.current.historyData).toEqual([{ timestamp: 3000, cpu: 31, memory: 61 }]);

    await act(async () => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS);
      await flushMicrotasks();
    });
    expect(result.current.monitorData?.instanceId).toBe('instance-b');
    expect(result.current.historyData).toEqual([{ timestamp: 4000, cpu: 40, memory: 70 }]);
  });

  it('does not fabricate a history point from the current snapshot', async () => {
    mockGetMonitor.mockResolvedValue({
      instanceId: 'instance-a',
      collectedAt: 5000,
      cpuUsage: 88,
      memoryUsagePercent: 77,
      history: [],
    });

    const { result } = renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });

    expect(result.current.monitorData?.cpuUsage).toBe(88);
    expect(result.current.historyData).toEqual([]);
    expect(result.current.lastUpdated).toBe(5000);
  });

  it('maps the server stale flag without turning it into a request error', async () => {
    mockGetMonitor.mockResolvedValue({
      instanceId: 'instance-a',
      collectedAt: 5000,
      stale: true,
      history: [{ timestamp: 5000, cpuUsage: 25, memoryUsagePercent: 50 }],
    });

    const { result } = renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });

    expect(result.current.stale).toBe(true);
    expect(result.current.error).toBeNull();
    expect(result.current.historyData).toEqual([{ timestamp: 5000, cpu: 25, memory: 50 }]);
  });

  it('retains the last successful snapshot and history when a later request fails', async () => {
    const snapshot = {
      instanceId: 'instance-a',
      collectedAt: 5000,
      cpuUsage: 25,
      history: [{ timestamp: 5000, cpuUsage: 25, memoryUsagePercent: 50 }],
    };
    mockGetMonitor.mockResolvedValueOnce(snapshot).mockRejectedValueOnce(new Error('network down'));

    const { result } = renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });
    expect(result.current.monitorData).toEqual(snapshot);

    await act(async () => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS);
      await flushMicrotasks();
    });

    expect(result.current.monitorData).toEqual(snapshot);
    expect(result.current.historyData).toEqual([{ timestamp: 5000, cpu: 25, memory: 50 }]);
    expect(result.current.lastUpdated).toBe(5000);
    expect(result.current.stale).toBe(true);
    expect(result.current.error?.message).toBe('network down');
  });

  it('polls every five seconds only while the page is visible', async () => {
    setDocumentVisibility('hidden');
    mockGetMonitor.mockResolvedValue({ history: [] });

    renderHook(() => useMonitorData());
    expect(mockGetMonitor).not.toHaveBeenCalled();

    act(() => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS * 3);
    });
    expect(mockGetMonitor).not.toHaveBeenCalled();

    setDocumentVisibility('visible');
    await act(async () => {
      document.dispatchEvent(new Event('visibilitychange'));
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);

    await act(async () => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(2);

    setDocumentVisibility('hidden');
    act(() => {
      document.dispatchEvent(new Event('visibilitychange'));
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS * 3);
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(2);
  });

  it('does not poll faster than the server sampling interval', async () => {
    mockGetMonitor.mockResolvedValue({
      sampleIntervalMs: 30_000,
      history: [],
    });

    renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);

    await act(async () => {
      jest.advanceTimersByTime(29_999);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);

    await act(async () => {
      jest.advanceTimersByTime(1);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(2);
  });

  it('ignores an out-of-range server sampling interval', async () => {
    mockGetMonitor.mockResolvedValue({
      sampleIntervalMs: MONITOR_MAX_SAMPLE_INTERVAL_MS + 1,
      history: [],
    });

    renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });

    await act(async () => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS - 1);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);

    await act(async () => {
      jest.advanceTimersByTime(1);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(2);
  });

  it('reports the initial 503 response as a not-ready state', async () => {
    mockGetMonitor.mockRejectedValue(
      Object.assign(new Error('service unavailable'), { response: { status: 503 } }),
    );

    const { result } = renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });

    expect(result.current.loading).toBe(false);
    expect(result.current.notReady).toBe(true);
    expect(result.current.monitorData).toBeNull();
    expect(result.current.historyData).toEqual([]);
  });

  it('uses Retry-After while the sampler is not ready', async () => {
    mockGetMonitor
      .mockRejectedValueOnce(
        Object.assign(new Error('service unavailable'), {
          response: {
            status: 503,
            headers: { get: (name: string) => (name === 'Retry-After' ? '30' : null) },
          },
        }),
      )
      .mockResolvedValue({ history: [] });

    renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });

    await act(async () => {
      jest.advanceTimersByTime(29_999);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);

    await act(async () => {
      jest.advanceTimersByTime(1);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(2);
  });

  it.each([
    ['zero', '0', MONITOR_POLL_INTERVAL_MS],
    ['excessive', '999999', MONITOR_MAX_RETRY_INTERVAL_MS],
  ] as const)('bounds a %s Retry-After value', async (_label, retryAfter, expectedDelay) => {
    mockGetMonitor
      .mockRejectedValueOnce(
        Object.assign(new Error('service unavailable'), {
          response: { status: 503, headers: { 'retry-after': retryAfter } },
        }),
      )
      .mockResolvedValue({ history: [] });

    renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });

    await act(async () => {
      jest.advanceTimersByTime(expectedDelay - 1);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);

    await act(async () => {
      jest.advanceTimersByTime(1);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(2);
  });

  it('returns to the base polling interval when a network error is followed by 503', async () => {
    mockGetMonitor
      .mockRejectedValueOnce(new Error('network unavailable'))
      .mockRejectedValueOnce(
        Object.assign(new Error('service unavailable'), { response: { status: 503 } }),
      )
      .mockResolvedValue({ history: [] });

    const { result } = renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);

    await act(async () => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);

    await act(async () => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(2);
    expect(result.current.notReady).toBe(true);

    await act(async () => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS - 1);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(2);

    await act(async () => {
      jest.advanceTimersByTime(1);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(3);
  });

  it('stops automatic polling and exposes permission denial after a 403', async () => {
    mockGetMonitor.mockRejectedValue(
      Object.assign(new Error('forbidden'), { response: { status: 403 } }),
    );

    const { result } = renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });

    expect(result.current.permissionDenied).toBe(true);
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);
    act(() => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS * 20);
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);
  });

  it('restarts automatic polling after a successful manual retry', async () => {
    mockGetMonitor
      .mockRejectedValueOnce(Object.assign(new Error('forbidden'), { response: { status: 403 } }))
      .mockResolvedValue({
        collectedAt: 1000,
        history: [{ timestamp: 1000, cpuUsage: 10, memoryUsagePercent: 20 }],
      });

    const { result } = renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });
    expect(result.current.permissionDenied).toBe(true);

    await act(async () => {
      result.current.refresh();
      await flushMicrotasks();
    });
    expect(result.current.permissionDenied).toBe(false);
    expect(mockGetMonitor).toHaveBeenCalledTimes(2);

    await act(async () => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS);
      await flushMicrotasks();
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(3);
  });

  it('clears an expired session, redirects, and stops polling after a 401', async () => {
    localStorage.setItem('token', 'expired');
    localStorage.setItem('token.expire', '1');
    localStorage.setItem('autoLogin', 'true');
    setTransientAuthToken('expired-memory-token');
    mockGetMonitor.mockRejectedValue(
      Object.assign(new Error('unauthorized'), { response: { status: 401 } }),
    );

    renderHook(() => useMonitorData());
    await act(async () => {
      await flushMicrotasks();
    });

    expect(localStorage.removeItem).toHaveBeenCalledWith('token');
    expect(localStorage.removeItem).toHaveBeenCalledWith('token.expire');
    expect(localStorage.removeItem).toHaveBeenCalledWith('autoLogin');
    expect(getAuthToken({ getItem: jest.fn(() => null) })).toBeNull();
    expect(mockHistoryPush).toHaveBeenCalledWith('/user/login');
    act(() => {
      jest.advanceTimersByTime(MONITOR_POLL_INTERVAL_MS * 20);
    });
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);
  });

  it('does not clear the session, redirect, or report an error when a request settles after unmount', async () => {
    let rejectRequest: ((reason: unknown) => void) | undefined;
    const onError = jest.fn();
    setTransientAuthToken('active-memory-token');
    mockGetMonitor.mockImplementationOnce(
      () =>
        new Promise((_resolve, reject) => {
          rejectRequest = reject;
        }),
    );

    const { unmount } = renderHook(() => useMonitorData({ onError }));
    expect(mockGetMonitor).toHaveBeenCalledTimes(1);
    unmount();

    await act(async () => {
      rejectRequest?.(Object.assign(new Error('unauthorized'), { response: { status: 401 } }));
      await flushMicrotasks();
    });

    expect(localStorage.removeItem).not.toHaveBeenCalled();
    expect(getAuthToken({ getItem: jest.fn(() => null) })).toBe('active-memory-token');
    expect(mockHistoryPush).not.toHaveBeenCalled();
    expect(onError).not.toHaveBeenCalled();
  });
});
