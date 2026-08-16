import BellOutlined from '@ant-design/icons/BellOutlined';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { history, useIntl } from '@umijs/max';
import { Badge, Grid, Popover } from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import { operationsAPI } from '@/modules/operations/api';
import type { NoticeSummary, NoticeType } from '@/modules/operations/contract';
import { hasPermission } from '@/shared/auth/access';
import type { CurrentUser } from '@/shared/auth/types';
import { queryKeys } from '@/shared/query/client';
import { useHeaderNoticeStyles } from './HeaderNotice.styles';
import HeaderNoticePanel, {
  HEADER_NOTICE_TYPES,
  type HeaderNoticeData,
  type HeaderNoticeLoadState,
  type HeaderNoticePanelHandle,
} from './HeaderNoticePanel';

const NOTICE_REFRESH_INTERVAL = 50_000;

function groupUnreadNotices(notices: NoticeSummary[] | undefined): HeaderNoticeData {
  const grouped: HeaderNoticeData = {
    event: [],
    mail: [],
    message: [],
    notification: [],
  };
  for (const notice of notices ?? []) {
    if (!notice.read) grouped[notice.type].push(notice);
  }
  return grouped;
}

export default function HeaderNotice({ user }: { user?: CurrentUser }) {
  const intl = useIntl();
  const { styles } = useHeaderNoticeStyles();
  const screens = Grid.useBreakpoint();
  const queryClient = useQueryClient();
  const canRead = hasPermission(user, '/notice');
  const canMarkRead = hasPermission(user, '/notice/read');
  const [open, setOpen] = useState(false);
  const [activeType, setActiveType] = useState<NoticeType>('notification');
  const panelRef = useRef<HeaderNoticePanelHandle>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const wasOpenRef = useRef(false);
  const unread = useQuery({
    queryKey: queryKeys.noticeUnread,
    queryFn: operationsAPI.notices.unread,
    enabled: canRead,
    refetchInterval: NOTICE_REFRESH_INTERVAL,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
    staleTime: 30_000,
  });
  const markRead = useMutation({
    mutationFn: (id: string) => operationsAPI.notices.markRead(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.notices });
    },
  });

  const noticeData = useMemo(() => groupUnreadNotices(unread.data), [unread.data]);
  const unreadCount = useMemo(
    () => HEADER_NOTICE_TYPES.reduce((count, type) => count + noticeData[type].length, 0),
    [noticeData],
  );

  useEffect(() => {
    if (open) {
      wasOpenRef.current = true;
      const animationFrame = window.requestAnimationFrame(() => panelRef.current?.focusFirst());
      return () => window.cancelAnimationFrame(animationFrame);
    }

    if (wasOpenRef.current) {
      triggerRef.current?.focus();
      wasOpenRef.current = false;
    }
    return undefined;
  }, [open]);

  if (!canRead) return null;

  const label = intl.formatMessage({ id: 'notice.header.label' });
  const loadState: HeaderNoticeLoadState =
    unread.isPending && !unread.data
      ? 'loading'
      : unread.isError && !unread.data
        ? 'error'
        : 'ready';

  return (
    <Popover
      arrow={false}
      content={
        <HeaderNoticePanel
          activeType={activeType}
          canMarkRead={canMarkRead}
          isRefreshError={unread.isError && Boolean(unread.data)}
          label={label}
          loadState={loadState}
          markReadError={markRead.error}
          markReadPending={markRead.isPending}
          noticeData={noticeData}
          ref={panelRef}
          onActiveTypeChange={(type) => {
            setActiveType(type);
            markRead.reset();
          }}
          onClose={() => setOpen(false)}
          onMarkRead={(id) => markRead.mutate(id)}
          onMarkReadErrorDismiss={() => markRead.reset()}
          onRetry={() => void unread.refetch()}
          onViewAll={(type) => {
            setOpen(false);
            history.push(`/notice?type=${type}`);
          }}
        />
      }
      destroyOnHidden
      open={open}
      placement={screens.sm === false ? 'bottom' : 'bottomRight'}
      styles={{
        container: {
          maxWidth: 'calc(100vw - 16px)',
          overflow: 'hidden',
          padding: 0,
          width: 380,
        },
      }}
      trigger="click"
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen);
        if (nextOpen) {
          markRead.reset();
          void unread.refetch();
        }
      }}
    >
      <button
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-label={label}
        className={styles.trigger}
        ref={triggerRef}
        type="button"
      >
        <Badge count={unreadCount} overflowCount={99} size="small">
          <BellOutlined />
        </Badge>
      </button>
    </Popover>
  );
}
