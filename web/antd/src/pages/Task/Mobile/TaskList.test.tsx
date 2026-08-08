import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { message } from 'antd';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import MobileTaskList, { resolveTaskOperation } from './TaskList';

const React = require('react');

jest.mock('@/hooks/useMobileListPagination', () => ({
  useMobileListPagination: jest.fn(),
}));

jest.mock('@/components/MssBoot/Access', () => ({
  Access: ({ children }: { children: React.ReactNode }) => children,
}));

const messages: Record<string, string> = {
  'pages.description.delete.confirm': 'Delete this record?',
  'pages.fields.checkedAt': 'Last execution time',
  'pages.fields.namespace': 'Namespace',
  'pages.fields.options.disabled': 'Disabled',
  'pages.fields.options.enabled': 'Enabled',
  'pages.fields.options.locked': 'Locked',
  'pages.fields.provider': 'Provider',
  'pages.mobile.loadFailed': 'Unable to load data',
  'pages.mobile.retry': 'Retry',
  'pages.table.new': 'New',
  'pages.task.start.success': 'Started',
  'pages.task.start.title': 'Start',
  'pages.task.stop.success': 'Stopped',
  'pages.task.stop.title': 'Stop',
  'pages.title.cancel': 'Cancel',
  'pages.title.delete': 'Delete',
  'pages.title.edit': 'Edit',
  'pages.title.ok': 'OK',
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
const disabledTask: API.Task = {
  id: 'task-1',
  name: 'Health check',
  provider: 'default',
  namespace: 'system',
  checkedAt: '2026-08-08T10:30:00Z',
  status: 'disabled',
};

const paginationResult = {
  dataSource: [disabledTask],
  error: undefined,
  hasMore: false,
  isClientSideFallback: false,
  loading: false,
  loadingMore: false,
  loadMore: jest.fn().mockResolvedValue(undefined),
  reload,
};

const renderList = (onOperate: (record: API.Task, operation: 'start' | 'stop') => Promise<void>) =>
  render(
    <MobileTaskList
      request={request}
      onCreate={jest.fn()}
      onEdit={jest.fn()}
      onDelete={jest.fn().mockResolvedValue(undefined)}
      onOperate={onOperate}
    />,
  );

describe('MobileTaskList', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedPagination.mockReturnValue(paginationResult);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('maps the backend status contract to the only valid operation', () => {
    expect(resolveTaskOperation('enabled')).toBe('stop');
    expect(resolveTaskOperation('disabled')).toBe('start');
    expect(resolveTaskOperation('locked')).toBeUndefined();
    expect(resolveTaskOperation('')).toBe('start');
  });

  it('uses localized fields and starts a disabled task only once while pending', async () => {
    let resolveOperate: (() => void) | undefined;
    const onOperate = jest.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveOperate = resolve;
        }),
    );
    const successSpy = jest.spyOn(message, 'success').mockImplementation(() => undefined as never);

    renderList(onOperate);

    expect(screen.getByText('Disabled')).toBeTruthy();
    expect(screen.getByText('Provider:')).toBeTruthy();
    expect(screen.getByText('Namespace:')).toBeTruthy();
    expect(screen.getByText('Last execution time:')).toBeTruthy();

    const startButton = screen.getByRole('button', { name: 'Start' });
    fireEvent.click(startButton);
    fireEvent.click(startButton);

    expect(onOperate).toHaveBeenCalledTimes(1);
    expect(onOperate).toHaveBeenCalledWith(disabledTask, 'start');
    expect(successSpy).not.toHaveBeenCalled();
    expect(reload).not.toHaveBeenCalled();

    await act(async () => {
      resolveOperate?.();
    });

    await waitFor(() => {
      expect(successSpy).toHaveBeenCalledWith('Started');
      expect(reload).toHaveBeenCalledTimes(1);
    });
  });

  it('offers stop for enabled tasks and no operation for locked tasks', () => {
    mockedPagination.mockReturnValue({
      ...paginationResult,
      dataSource: [
        { ...disabledTask, id: 'task-enabled', status: 'enabled' },
        { ...disabledTask, id: 'task-locked', status: 'locked' },
      ] satisfies API.Task[],
    });

    renderList(jest.fn().mockResolvedValue(undefined));

    expect(screen.getByRole('button', { name: 'Stop' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Start' })).toBeNull();
    expect(screen.getByText('Locked')).toBeTruthy();
  });

  it('does not report success or reload when an operation fails', async () => {
    const successSpy = jest.spyOn(message, 'success').mockImplementation(() => undefined as never);
    renderList(jest.fn().mockRejectedValue(new Error('scheduler unavailable')));

    fireEvent.click(screen.getByRole('button', { name: 'Start' }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Start' })).toBeTruthy();
    });
    expect(successSpy).not.toHaveBeenCalled();
    expect(reload).not.toHaveBeenCalled();
  });

  it('shows a retryable error instead of an empty list after an initial failure', () => {
    mockedPagination.mockReturnValue({
      ...paginationResult,
      dataSource: [],
      error: new Error('offline'),
    });

    renderList(jest.fn().mockResolvedValue(undefined));

    expect(screen.getByText('Unable to load data')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(reload).toHaveBeenCalledTimes(1);
  });
});
