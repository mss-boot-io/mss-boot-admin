import { act, render } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import SessionRefreshBridge from './SessionRefreshBridge';

const state = vi.hoisted(() => ({
  expiry: undefined as number | undefined,
  initialState: { currentUser: { id: 'user-1' } } as unknown,
  refresh: vi.fn(),
}));

vi.mock('@umijs/max', () => ({
  useModel: () => ({ initialState: state.initialState }),
}));

vi.mock('./session', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./session')>();
  return {
    ...actual,
    clearBrowserSessionMetadata: vi.fn(),
    readBrowserSessionExpiry: () => state.expiry,
    redirectToLogin: vi.fn(),
    refreshBrowserSessionIfDue: state.refresh,
  };
});

describe('browser session refresh scheduling', () => {
  const originalVisibility = Object.getOwnPropertyDescriptor(document, 'visibilityState');

  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-08-16T00:00:00Z'));
    state.expiry = undefined;
    state.refresh.mockReset();
    state.refresh.mockImplementation(async () => {
      state.expiry = Date.now() + 12 * 60 * 60 * 1000;
      return state.expiry;
    });
    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      value: 'visible',
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    if (originalVisibility) Object.defineProperty(document, 'visibilityState', originalVisibility);
    else Reflect.deleteProperty(document, 'visibilityState');
  });

  it('establishes missing expiry metadata and schedules the next pre-expiry refresh', async () => {
    render(<SessionRefreshBridge />);

    await act(async () => vi.advanceTimersByTimeAsync(0));
    expect(state.refresh).toHaveBeenCalledTimes(1);

    await act(async () => vi.advanceTimersByTimeAsync(11 * 60 * 60 * 1000));
    expect(state.refresh).toHaveBeenCalledTimes(1);
    await act(async () => vi.advanceTimersByTimeAsync(56 * 60 * 1000));
    expect(state.refresh).toHaveBeenCalledTimes(2);
  });

  it('waits while hidden and refreshes when an authenticated tab becomes visible', async () => {
    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      value: 'hidden',
    });
    render(<SessionRefreshBridge />);
    await act(async () => vi.advanceTimersByTimeAsync(60_000));
    expect(state.refresh).not.toHaveBeenCalled();

    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      value: 'visible',
    });
    document.dispatchEvent(new Event('visibilitychange'));
    await act(async () => vi.advanceTimersByTimeAsync(0));
    expect(state.refresh).toHaveBeenCalledTimes(1);
  });
});
