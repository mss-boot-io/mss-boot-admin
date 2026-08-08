import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { message } from 'antd';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import MobileNoticeList from './NoticeList';

const React = require('react');

jest.mock('@/hooks/useMobileListPagination', () => ({
  useMobileListPagination: jest.fn(),
}));

jest.mock('@/components/MssBoot/Access', () => ({
  Access: ({ children }: { children: React.ReactNode }) => children,
}));

const messages: Record<string, string> = {
  'component.noticeIcon.empty': 'No notifications',
  'pages.fields.createdAt': 'Created time',
  'pages.fields.description': 'Description',
  'pages.fields.options.email': 'Email',
  'pages.fields.options.notification': 'Notification',
  'pages.fields.options.todo': 'Todo',
  'pages.fields.options.urgent': 'Urgent',
  'pages.title.notice.read': 'Mark as read',
  'pages.mobile.loadFailed': 'Unable to load data',
  'pages.mobile.retry': 'Retry',
};

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatDate: () => '08/08 10:30',
    formatMessage: ({ id, defaultMessage }: { id: string; defaultMessage?: string }) =>
      messages[id] || defaultMessage || id,
  }),
}));

const mockedPagination = useMobileListPagination as jest.Mock;
const reload = jest.fn().mockResolvedValue(undefined);
const request = jest.fn().mockResolvedValue({ data: [] });

const paginationResult = {
  dataSource: [
    {
      id: 'notice-1',
      title: 'Service warning',
      description: 'CPU usage is elevated.',
      type: 'notification',
      status: 'urgent',
      read: false,
      createdAt: '2026-08-08T10:30:00Z',
    },
    {
      id: 'notice-2',
      title: 'Daily report',
      description: 'The daily report is ready.',
      type: 'mail',
      status: 'todo',
      read: true,
    },
  ] satisfies API.Notice[],
  error: undefined,
  hasMore: false,
  isClientSideFallback: false,
  loading: false,
  loadingMore: false,
  loadMore: jest.fn().mockResolvedValue(undefined),
  reload,
};

describe('MobileNoticeList', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedPagination.mockReturnValue(paginationResult);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('renders a read-only inbox with API notice types and opens details from the title', () => {
    const onView = jest.fn();

    render(
      <MobileNoticeList
        request={request}
        onView={onView}
        onMarkRead={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    expect(screen.getByText('Notification')).toBeTruthy();
    expect(screen.getByText('Email')).toBeTruthy();
    expect(screen.getByText('Urgent')).toBeTruthy();
    expect(screen.getByText('Todo')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'New' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Edit' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Delete' })).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Service warning' }));
    expect(onView).toHaveBeenCalledWith(expect.objectContaining({ id: 'notice-1' }));
  });

  it('marks one unread notice only once and reloads only after the request succeeds', async () => {
    let resolveMarkRead: (() => void) | undefined;
    const onMarkRead = jest.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveMarkRead = resolve;
        }),
    );
    const successSpy = jest.spyOn(message, 'success').mockImplementation(() => undefined as never);

    render(
      <MobileNoticeList request={request} onView={jest.fn()} onMarkRead={onMarkRead} />,
    );

    const markReadButton = screen.getByRole('button', { name: 'Mark as read' });
    fireEvent.click(markReadButton);
    fireEvent.click(markReadButton);

    expect(onMarkRead).toHaveBeenCalledTimes(1);
    expect(successSpy).not.toHaveBeenCalled();
    expect(reload).not.toHaveBeenCalled();

    await act(async () => {
      resolveMarkRead?.();
    });

    await waitFor(() => {
      expect(successSpy).toHaveBeenCalledWith('Mark as read');
      expect(reload).toHaveBeenCalledTimes(1);
    });
  });

  it('does not report success or reload when marking as read fails', async () => {
    const successSpy = jest.spyOn(message, 'success').mockImplementation(() => undefined as never);

    render(
      <MobileNoticeList
        request={request}
        onView={jest.fn()}
        onMarkRead={jest.fn().mockRejectedValue(new Error('offline'))}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Mark as read' }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Mark as read' })).toBeTruthy();
    });
    expect(successSpy).not.toHaveBeenCalled();
    expect(reload).not.toHaveBeenCalled();
  });

  it('shows a retryable error without also claiming the inbox is empty', () => {
    mockedPagination.mockReturnValue({
      ...paginationResult,
      dataSource: [],
      error: new Error('offline'),
    });

    render(
      <MobileNoticeList
        request={request}
        onView={jest.fn()}
        onMarkRead={jest.fn().mockResolvedValue(undefined)}
      />,
    );

    expect(screen.getByText('Unable to load data')).toBeTruthy();
    expect(screen.queryByText('No notifications')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(reload).toHaveBeenCalledTimes(1);
  });
});
