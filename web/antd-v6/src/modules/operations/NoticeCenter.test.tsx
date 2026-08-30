import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen } from '@testing-library/react';
import { App } from 'antd';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resolveEffectivePagePresentation } from '../../shared/presentation/runtime';
import NoticeCenter from './NoticeCenter';
import { noticePresentationRegistryEntry } from './tablePresentation';

const noticePage = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'en-US',
    formatMessage: ({ id }: { id: string }) => id,
  }),
  useSearchParams: () => [new URLSearchParams('type=message')],
}));

vi.mock('./query', () => ({
  useNotice: () => ({ data: undefined, error: null, isError: false, isPending: false }),
  useNoticePage: () => noticePage.current,
}));

vi.mock('./api', () => ({
  operationsAPI: { notices: { markRead: vi.fn() } },
}));

function renderCenter() {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  return render(
    <App>
      <QueryClientProvider client={client}>
        <NoticeCenter
          canMarkRead
          presentationRuntime={resolveEffectivePagePresentation({
            entry: noticePresentationRegistryEntry,
            locale: 'en-US',
            settled: true,
          })}
        />
      </QueryClientProvider>
    </App>,
  );
}

describe('notice center route filters', () => {
  beforeEach(() => {
    noticePage.current = {
      data: undefined,
      error: null,
      isError: false,
      isFetching: true,
      isPending: true,
      refetch: vi.fn(),
    };
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('does not update the form instance before the loading state mounts it', async () => {
    vi.useFakeTimers();
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    renderCenter();
    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    expect(screen.getByRole('status')).toBeTruthy();
    expect(consoleError.mock.calls.flat().join(' ')).not.toContain(
      'Instance created by `useForm` is not connected to any Form element',
    );
  });
});
