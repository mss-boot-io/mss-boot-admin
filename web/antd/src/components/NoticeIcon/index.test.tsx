import { act, fireEvent, render, screen } from '@testing-library/react';
import NoticeIconView from './index';
import { getNoticeUnread, putNoticeReadId } from '@/services/admin/notice';
import { clearTransientAuthToken, setTransientAuthToken } from '@/utils/authStorage';

const ReactRuntime = require('react');

jest.mock('@umijs/max', () => ({
  history: { push: jest.fn() },
  useIntl: () => ({
    formatMessage: ({ defaultMessage, id }: { defaultMessage?: string; id: string }) =>
      defaultMessage || id,
  }),
}));

jest.mock('antd', () => {
  const ReactRuntime = require('react');

  return {
    message: { success: jest.fn() },
    Tag: ({ children }: { children?: unknown }) =>
      ReactRuntime.createElement(ReactRuntime.Fragment, null, children),
  };
});

jest.mock('./NoticeIcon', () => {
  const ReactRuntime = require('react');
  const MockNoticeIcon: any = ({
    count,
    onClear,
    onItemClick,
    onRetry,
    loading,
    loadingText,
    errorText,
    children,
  }: any) =>
    ReactRuntime.createElement(
      'div',
      null,
      ReactRuntime.createElement('span', { 'data-testid': 'notice-count' }, count),
      ReactRuntime.createElement(
        'span',
        { 'data-testid': 'notice-loading' },
        loading ? loadingText : '',
      ),
      ReactRuntime.createElement('span', { 'data-testid': 'notice-error' }, errorText || ''),
      ReactRuntime.createElement('button', { type: 'button', onClick: onRetry }, 'retry notices'),
      ReactRuntime.createElement(
        'button',
        { type: 'button', onClick: () => onItemClick?.({ id: 'notice-1' }) },
        'mark read',
      ),
      ReactRuntime.createElement(
        'button',
        { type: 'button', onClick: () => onClear?.('Notifications', 'notification') },
        'clear',
      ),
      loading || errorText ? null : children,
    );

  MockNoticeIcon.Tab = ({ count, tabKey }: { count?: number; tabKey: string }) =>
    ReactRuntime.createElement('span', { 'data-testid': `notice-tab-${tabKey}` }, count ?? 0);

  return { __esModule: true, default: MockNoticeIcon };
});

jest.mock('@/services/admin/notice', () => ({
  getNoticeUnread: jest.fn(),
  putNoticeReadId: jest.fn(),
}));

const mockGetNoticeUnread = getNoticeUnread as jest.Mock;
const mockPutNoticeReadId = putNoticeReadId as jest.Mock;

const setDocumentHidden = (hidden: boolean) => {
  Object.defineProperty(document, 'hidden', { configurable: true, value: hidden });
  document.dispatchEvent(new Event('visibilitychange'));
};

const flushMicrotasks = async () => {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
};

describe('NoticeIconView refresh behavior', () => {
  beforeEach(() => {
    jest.useFakeTimers();
    jest.clearAllMocks();
    Reflect.deleteProperty(document, 'hidden');
    setTransientAuthToken('oauth-admin-token');
  });

  afterEach(() => {
    clearTransientAuthToken();
    Reflect.deleteProperty(document, 'hidden');
    jest.useRealTimers();
  });

  it('pauses polling while the page is hidden and refreshes after it becomes visible', async () => {
    mockGetNoticeUnread.mockResolvedValue([]);
    setDocumentHidden(true);

    render(ReactRuntime.createElement(NoticeIconView));

    expect(mockGetNoticeUnread).not.toHaveBeenCalled();

    act(() => {
      setDocumentHidden(false);
    });
    await act(async () => {
      await flushMicrotasks();
    });

    expect(mockGetNoticeUnread).toHaveBeenCalledTimes(1);

    act(() => {
      setDocumentHidden(true);
      jest.advanceTimersByTime(3 * 50000);
    });

    expect(mockGetNoticeUnread).toHaveBeenCalledTimes(1);

    act(() => {
      setDocumentHidden(false);
    });

    expect(mockGetNoticeUnread).toHaveBeenCalledTimes(2);
  });

  it('shares an in-flight refresh and reads again after marking a notification as read', async () => {
    let resolveInitialRequest: ((value: API.Notice[]) => void) | undefined;
    const initialRequest = new Promise<API.Notice[]>((resolve) => {
      resolveInitialRequest = resolve;
    });
    mockGetNoticeUnread.mockImplementationOnce(() => initialRequest).mockResolvedValueOnce([]);
    mockPutNoticeReadId.mockResolvedValue({});

    render(ReactRuntime.createElement(NoticeIconView));

    expect(mockGetNoticeUnread).toHaveBeenCalledTimes(1);

    act(() => {
      jest.advanceTimersByTime(50000);
    });

    expect(mockGetNoticeUnread).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: 'mark read' }));
    await act(async () => {
      await flushMicrotasks();
    });

    expect(mockPutNoticeReadId).toHaveBeenCalledWith({ id: 'notice-1' });
    expect(mockGetNoticeUnread).toHaveBeenCalledTimes(1);

    await act(async () => {
      if (!resolveInitialRequest) {
        throw new Error('Expected the initial notification request to be pending');
      }

      resolveInitialRequest([]);
      await flushMicrotasks();
    });

    expect(mockGetNoticeUnread).toHaveBeenCalledTimes(2);
  });

  it('does not burst extra requests during rapid visibility changes', async () => {
    mockGetNoticeUnread.mockResolvedValue([]);

    render(ReactRuntime.createElement(NoticeIconView));
    await act(async () => {
      await flushMicrotasks();
    });

    for (let index = 0; index < 5; index += 1) {
      act(() => {
        setDocumentHidden(true);
        setDocumentHidden(false);
      });
    }

    expect(mockGetNoticeUnread).toHaveBeenCalledTimes(1);
  });

  it('renders mail notices in a matching tab so the badge count stays explainable', async () => {
    mockGetNoticeUnread.mockResolvedValue([{ id: 'mail-1', type: 'mail' }]);

    render(ReactRuntime.createElement(NoticeIconView));
    await act(async () => {
      await flushMicrotasks();
    });

    expect(screen.getByTestId('notice-count').textContent).toBe('1');
    expect(screen.getByTestId('notice-tab-mail').textContent).toBe('1');
  });

  it('keeps empty tabs hidden while loading and exposes a retry after the initial request fails', async () => {
    let rejectInitialRequest: ((reason?: unknown) => void) | undefined;
    mockGetNoticeUnread
      .mockImplementationOnce(
        () =>
          new Promise<API.Notice[]>((_resolve, reject) => {
            rejectInitialRequest = reject;
          }),
      )
      .mockResolvedValueOnce([]);

    render(ReactRuntime.createElement(NoticeIconView));

    expect(screen.getByTestId('notice-loading').textContent).toBe('Loading notifications…');
    expect(screen.queryByTestId('notice-tab-notification')).toBeNull();

    await act(async () => {
      rejectInitialRequest?.(new Error('offline'));
      await flushMicrotasks();
    });

    expect(screen.getByTestId('notice-error').textContent).toBe('Unable to load notifications');
    expect(screen.queryByTestId('notice-tab-notification')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'retry notices' }));
    await act(async () => {
      await flushMicrotasks();
    });

    expect(mockGetNoticeUnread).toHaveBeenCalledTimes(2);
    expect(screen.getByTestId('notice-tab-notification')).toBeTruthy();
  });
});
