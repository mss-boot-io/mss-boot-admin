import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { NoticeSummary, NoticeType } from '@/modules/operations/contract';
import type { CurrentUser } from '@/shared/auth/types';
import HeaderNotice from './HeaderNotice';

const { markRead, push, unread } = vi.hoisted(() => ({
  markRead: vi.fn(),
  push: vi.fn(),
  unread: vi.fn(),
}));

const messages: Record<string, string> = {
  'actions.retry': '重试',
  'notice.header.empty.event': '你已完成所有待办',
  'notice.header.empty.mail': '你已查看所有邮件',
  'notice.header.empty.message': '你已读完所有消息',
  'notice.header.empty.notification': '你已查看所有通知',
  'notice.header.label': '通知中心',
  'notice.header.loadFailed': '无法加载通知',
  'notice.header.loading': '正在加载通知…',
  'notice.header.markRead': '将“{title}”标记为已读',
  'notice.header.viewAll': '查看更多',
  'notice.read.failed': '更新通知状态失败',
  'notice.type.event': '事件',
  'notice.type.mail': '邮件',
  'notice.type.message': '消息',
  'notice.type.notification': '通知',
  'operations.refreshFailed': '刷新失败',
};

vi.mock('@umijs/max', () => ({
  history: { push },
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ id }: { id: string }, values: Record<string, number | string> = {}) => {
      let message = messages[id] ?? id;
      for (const [key, value] of Object.entries(values)) {
        message = message.replace(`{${key}}`, String(value));
      }
      return message;
    },
  }),
}));

vi.mock('@/modules/operations/api', () => ({
  operationsAPI: { notices: { markRead, unread } },
}));

function currentUser(canRead: boolean, canMarkRead = true): CurrentUser {
  return {
    id: 'user-1',
    permissions: {
      '/notice': canRead,
      '/notice/read': canMarkRead,
    },
  };
}

function notice(id: string, title: string, type: NoticeType): NoticeSummary {
  return {
    avatar: '',
    createdAt: '2026-08-16T08:00:00Z',
    datetime: '2026-08-16T08:00:00Z',
    description: `${title}的详细内容`,
    extra: type === 'event' ? '待处理' : '',
    id,
    key: id,
    read: false,
    status: type === 'event' ? 'todo' : '',
    title,
    type,
    updatedAt: '2026-08-16T08:00:00Z',
  };
}

function renderNotice(user: CurrentUser) {
  const client = new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false },
    },
  });
  return {
    client,
    ...render(
      <QueryClientProvider client={client}>
        <HeaderNotice user={user} />
      </QueryClientProvider>,
    ),
  };
}

beforeEach(() => {
  markRead.mockReset();
  push.mockReset();
  unread.mockReset();
});

describe('header notice popup', () => {
  it('groups unread notices in V5-style tabs and opens the selected type in the full center', async () => {
    unread.mockResolvedValue([
      notice('notice-1', '系统维护', 'notification'),
      notice('notice-2', '审批提醒', 'message'),
    ]);

    renderNotice(currentUser(true));

    const trigger = await screen.findByRole('button', { name: '通知中心' });
    expect(trigger.getAttribute('aria-haspopup')).toBe('dialog');
    expect(await screen.findByText('2')).toBeTruthy();

    fireEvent.click(trigger);
    expect(await screen.findByRole('dialog', { name: '通知中心' })).toBeTruthy();
    expect(screen.getByRole('tab', { name: /通知 \(1\)/ })).toBeTruthy();
    expect(screen.getByRole('tab', { name: /消息 \(1\)/ })).toBeTruthy();
    expect(screen.getByText('系统维护')).toBeTruthy();

    fireEvent.click(screen.getByRole('tab', { name: /消息 \(1\)/ }));
    expect(await screen.findByText('审批提醒')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '查看更多' }));

    expect(push).toHaveBeenCalledWith('/notice?type=message');
    await waitFor(() => expect(trigger.getAttribute('aria-expanded')).toBe('false'));
  });

  it('marks an unread item through the dedicated permission and refreshes notice queries', async () => {
    unread.mockResolvedValue([notice('notice-1', '系统维护', 'notification')]);
    markRead.mockResolvedValue(undefined);

    renderNotice(currentUser(true, true));
    fireEvent.click(await screen.findByRole('button', { name: '通知中心' }));
    fireEvent.click(await screen.findByRole('button', { name: '将“系统维护”标记为已读' }));

    await waitFor(() => expect(markRead).toHaveBeenCalledWith('notice-1'));
    await waitFor(() => expect(unread.mock.calls.length).toBeGreaterThanOrEqual(3));
  });

  it('keeps notices read-only when mark-read permission is absent', async () => {
    unread.mockResolvedValue([notice('notice-1', '系统维护', 'notification')]);

    renderNotice(currentUser(true, false));
    fireEvent.click(await screen.findByRole('button', { name: '通知中心' }));

    expect(await screen.findByText('系统维护')).toBeTruthy();
    expect(screen.queryByRole('button', { name: '将“系统维护”标记为已读' })).toBeNull();
  });

  it('shows a retryable load failure and recovers to the confirmed empty state', async () => {
    unread.mockRejectedValue(new Error('offline'));

    renderNotice(currentUser(true));
    fireEvent.click(await screen.findByRole('button', { name: '通知中心' }));

    expect((await screen.findByRole('alert')).textContent).toContain('无法加载通知');
    unread.mockResolvedValue([]);
    fireEvent.click(screen.getByRole('button', { name: /重\s*试/ }));

    expect(await screen.findByText('你已查看所有通知')).toBeTruthy();
  });

  it('closes on Escape and restores focus to the bell trigger', async () => {
    unread.mockResolvedValue([]);

    renderNotice(currentUser(true));
    const trigger = await screen.findByRole('button', { name: '通知中心' });
    fireEvent.click(trigger);
    const dialog = await screen.findByRole('dialog', { name: '通知中心' });

    fireEvent.keyDown(dialog, { key: 'Escape' });

    await waitFor(() => expect(trigger.getAttribute('aria-expanded')).toBe('false'));
    await waitFor(() => expect(document.activeElement).toBe(trigger));
  });

  it('does not expose or fetch the entry without notice permission', () => {
    renderNotice(currentUser(false));

    expect(screen.queryByRole('button', { name: '通知中心' })).toBeNull();
    expect(unread).not.toHaveBeenCalled();
  });
});
