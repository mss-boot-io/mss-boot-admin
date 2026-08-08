import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { message } from 'antd';
import { postTaskOperateId } from '@/services/admin/task';
import TaskList from './index';

const React = require('react');

const mockReload = jest.fn().mockResolvedValue(undefined);
const mockTasks: API.Task[] = [
  { id: 'task-disabled', name: 'Disabled task', status: 'disabled' },
  { id: 'task-enabled', name: 'Enabled task', status: 'enabled' },
];

jest.mock('@/components/MssBoot/Access', () => ({
  Access: ({ children }: { children: React.ReactNode }) => children,
}));

jest.mock('@/hooks/useResponsive', () => ({
  useResponsive: () => ({ isMobile: false }),
}));

jest.mock('@/services/admin/task', () => ({
  deleteTasksId: jest.fn(),
  getTasks: jest.fn(),
  getTasksId: jest.fn(),
  postTaskOperateId: jest.fn(),
  postTasks: jest.fn(),
  putTasksId: jest.fn(),
}));

jest.mock('@/util/columnOptions', () => ({ idRender: jest.fn() }));
jest.mock('@/util/fieldIntl', () => ({ fieldIntl: (_intl: unknown, id: string) => id }));
jest.mock('@/util/indexTitle', () => ({ indexTitle: jest.fn(() => 'Tasks') }));
jest.mock('@/utils/routeAccess', () => ({ resolveCrudRouteID: jest.fn() }));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ id, defaultMessage }: { id: string; defaultMessage?: string }) => (
    <>{defaultMessage || id}</>
  ),
  Link: ({ children, to }: { children: React.ReactNode; to: string }) => (
    <a href={to}>{children}</a>
  ),
  history: { location: { pathname: '/task' }, push: jest.fn() },
  useIntl: () => ({
    formatMessage: ({ id, defaultMessage }: { id: string; defaultMessage?: string }) =>
      defaultMessage || id,
  }),
  useParams: () => ({}),
}));

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  ProDescriptions: () => null,
  ProFormSelect: () => null,
  ProTable: (props: any) => {
    props.actionRef.current = { reload: mockReload };
    const operationColumn = props.columns.find(
      (column: { dataIndex?: string }) => column.dataIndex === 'option',
    );
    return <>{mockTasks.map((task) => operationColumn.render(null, task))}</>;
  },
}));

describe('TaskList desktop operations', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('posts only once for a rapid double click and locks other task operations while pending', async () => {
    let resolvePost: (() => void) | undefined;
    (postTaskOperateId as jest.Mock).mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolvePost = resolve;
        }),
    );
    const successSpy = jest.spyOn(message, 'success').mockImplementation(() => undefined as never);

    render(<TaskList />);

    const startButton = screen.getByRole('button', { name: 'Start' });
    act(() => {
      fireEvent.click(startButton);
      fireEvent.click(startButton);
    });

    expect(postTaskOperateId).toHaveBeenCalledTimes(1);
    expect(postTaskOperateId).toHaveBeenCalledWith({
      id: 'task-disabled',
      operate: 'start',
    });
    expect(startButton.getAttribute('class')).toContain('ant-btn-loading');
    expect((screen.getByRole('button', { name: 'Stop' }) as HTMLButtonElement).disabled).toBe(true);
    expect(successSpy).not.toHaveBeenCalled();
    expect(mockReload).not.toHaveBeenCalled();

    await act(async () => {
      resolvePost?.();
    });

    await waitFor(() => {
      expect(successSpy).toHaveBeenCalledWith('Start successfully!');
      expect(mockReload).toHaveBeenCalledTimes(1);
    });
  });
});
