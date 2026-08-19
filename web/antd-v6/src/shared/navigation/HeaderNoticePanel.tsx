import type {
  NoticeStatus,
  NoticeSummary,
  NoticeType,
} from '@mss-admin-core/modules/operations/contract';
import { getRequestErrorMessage } from '@mss-admin-core/shared/api/errors';
import { useIntl } from '@umijs/max';
import { Alert, Avatar, Button, Empty, Spin, Tabs, Tag, Typography } from 'antd';
import { forwardRef, type KeyboardEvent, useImperativeHandle, useRef } from 'react';
import { useHeaderNoticeStyles } from './HeaderNotice.styles';

export const HEADER_NOTICE_TYPES = ['notification', 'message', 'event', 'mail'] as const;

const NOTICE_FOCUSABLE_SELECTOR =
  '[role="tab"][tabindex="0"], button:not([disabled]):not([tabindex="-1"]), a[href], [tabindex]:not([tabindex="-1"]):not([role="tabpanel"])';

const STATUS_COLORS: Record<Exclude<NoticeStatus, ''>, string> = {
  doing: 'gold',
  processing: 'blue',
  todo: 'default',
  urgent: 'red',
};

export type HeaderNoticeData = Record<NoticeType, NoticeSummary[]>;
export type HeaderNoticeLoadState = 'error' | 'loading' | 'ready';

export interface HeaderNoticePanelHandle {
  focusFirst: () => void;
}

interface HeaderNoticePanelProps {
  activeType: NoticeType;
  canMarkRead: boolean;
  isRefreshError: boolean;
  label: string;
  loadState: HeaderNoticeLoadState;
  markReadError: unknown;
  markReadPending: boolean;
  noticeData: HeaderNoticeData;
  onActiveTypeChange: (type: NoticeType) => void;
  onClose: () => void;
  onMarkRead: (id: string) => void;
  onMarkReadErrorDismiss: () => void;
  onRetry: () => void;
  onViewAll: (type: NoticeType) => void;
}

function getFocusableElements(dialog: HTMLDivElement | null): HTMLElement[] {
  if (!dialog) return [];
  return Array.from(dialog.querySelectorAll<HTMLElement>(NOTICE_FOCUSABLE_SELECTOR)).filter(
    (element) => {
      const computedStyle = window.getComputedStyle(element);
      return (
        !element.closest('[aria-hidden="true"]') &&
        computedStyle.display !== 'none' &&
        computedStyle.visibility !== 'hidden'
      );
    },
  );
}

function formatNoticeTime(value: string | undefined, locale: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';

  const difference = date.getTime() - Date.now();
  const absoluteDifference = Math.abs(difference);
  const relative = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });
  if (absoluteDifference < 60_000) return relative.format(Math.round(difference / 1_000), 'second');
  if (absoluteDifference < 3_600_000)
    return relative.format(Math.round(difference / 60_000), 'minute');
  if (absoluteDifference < 86_400_000)
    return relative.format(Math.round(difference / 3_600_000), 'hour');
  if (absoluteDifference < 604_800_000)
    return relative.format(Math.round(difference / 86_400_000), 'day');

  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date);
}

function formatAbsoluteTime(value: string | undefined, locale: string): string | undefined {
  if (!value) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return undefined;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date);
}

const HeaderNoticePanel = forwardRef<HeaderNoticePanelHandle, HeaderNoticePanelProps>(
  function HeaderNoticePanel(
    {
      activeType,
      canMarkRead,
      isRefreshError,
      label,
      loadState,
      markReadError,
      markReadPending,
      noticeData,
      onActiveTypeChange,
      onClose,
      onMarkRead,
      onMarkReadErrorDismiss,
      onRetry,
      onViewAll,
    },
    ref,
  ) {
    const intl = useIntl();
    const { styles } = useHeaderNoticeStyles();
    const dialogRef = useRef<HTMLDivElement>(null);
    const locale = intl.locale || 'zh-CN';

    useImperativeHandle(ref, () => ({
      focusFirst: () => {
        const [firstFocusable] = getFocusableElements(dialogRef.current);
        (firstFocusable ?? dialogRef.current)?.focus();
      },
    }));

    const renderNotice = (notice: NoticeSummary) => {
      const timestamp = notice.datetime || notice.createdAt;
      const content = (
        <>
          <Avatar className={styles.avatar} src={notice.avatar || undefined}>
            {notice.avatar ? null : notice.title.slice(0, 1)}
          </Avatar>
          <div className={styles.itemBody}>
            <div className={styles.titleRow}>
              <Typography.Text className={styles.title} ellipsis>
                {notice.title}
              </Typography.Text>
              {notice.extra ? (
                <Tag
                  className={styles.extra}
                  color={notice.status ? STATUS_COLORS[notice.status] : undefined}
                >
                  {notice.extra}
                </Tag>
              ) : null}
            </div>
            {notice.description ? (
              <div className={styles.description}>{notice.description}</div>
            ) : null}
            {timestamp ? (
              <div className={styles.datetime} title={formatAbsoluteTime(timestamp, locale)}>
                {formatNoticeTime(timestamp, locale)}
              </div>
            ) : null}
          </div>
        </>
      );

      return (
        <li className={styles.item} key={notice.id}>
          {canMarkRead ? (
            <button
              aria-label={intl.formatMessage(
                { id: 'notice.header.markRead' },
                { title: notice.title },
              )}
              className={styles.itemAction}
              disabled={markReadPending}
              type="button"
              onClick={() => onMarkRead(notice.id)}
            >
              {content}
            </button>
          ) : (
            <div className={styles.itemReadOnly}>{content}</div>
          )}
        </li>
      );
    };

    const renderTabContent = (type: NoticeType) => {
      if (loadState === 'loading') {
        return (
          <div aria-live="polite" className={styles.state} role="status">
            <div className={styles.loading}>
              <Spin size="small" />
              <span>{intl.formatMessage({ id: 'notice.header.loading' })}</span>
            </div>
          </div>
        );
      }

      if (loadState === 'error') {
        return (
          <div className={styles.state}>
            <Alert
              className={styles.error}
              description={
                <Button size="small" type="primary" onClick={onRetry}>
                  {intl.formatMessage({ id: 'actions.retry' })}
                </Button>
              }
              role="alert"
              showIcon
              title={intl.formatMessage({ id: 'notice.header.loadFailed' })}
              type="error"
            />
          </div>
        );
      }

      const notices = noticeData[type];
      if (notices.length === 0) {
        return (
          <div className={styles.state}>
            <Empty
              description={intl.formatMessage({ id: `notice.header.empty.${type}` })}
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          </div>
        );
      }

      return <ul className={styles.list}>{notices.map(renderNotice)}</ul>;
    };

    const tabs = HEADER_NOTICE_TYPES.map((type) => {
      const title = intl.formatMessage({ id: `notice.type.${type}` });
      const count = noticeData[type].length;
      return {
        key: type,
        label: count > 0 ? `${title} (${count})` : title,
        children: renderTabContent(type),
      };
    });

    const handleDialogKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopPropagation();
        onClose();
        return;
      }

      if (event.key !== 'Tab') return;
      const focusableElements = getFocusableElements(dialogRef.current);
      if (focusableElements.length === 0) {
        event.preventDefault();
        dialogRef.current?.focus();
        return;
      }

      const currentIndex = focusableElements.indexOf(document.activeElement as HTMLElement);
      const nextIndex = event.shiftKey
        ? currentIndex <= 0
          ? focusableElements.length - 1
          : currentIndex - 1
        : currentIndex < 0 || currentIndex === focusableElements.length - 1
          ? 0
          : currentIndex + 1;
      event.preventDefault();
      event.stopPropagation();
      focusableElements[nextIndex]?.focus();
    };

    return (
      <div
        aria-label={label}
        className={styles.panel}
        ref={dialogRef}
        role="dialog"
        tabIndex={-1}
        onKeyDown={handleDialogKeyDown}
      >
        {isRefreshError ? (
          <Alert
            className={styles.refreshAlert}
            showIcon
            title={intl.formatMessage({ id: 'operations.refreshFailed' })}
            type="warning"
          />
        ) : null}
        {markReadError ? (
          <Alert
            closable
            className={styles.refreshAlert}
            description={getRequestErrorMessage(markReadError)}
            showIcon
            title={intl.formatMessage({ id: 'notice.read.failed' })}
            type="error"
            onClose={onMarkReadErrorDismiss}
          />
        ) : null}
        <Tabs
          activeKey={activeType}
          centered
          items={tabs}
          styles={{
            body: { minHeight: 248 },
            header: { margin: 0, paddingInline: 12 },
          }}
          onChange={(key) => onActiveTypeChange(key as NoticeType)}
        />
        <div className={styles.footer}>
          <Button block type="text" onClick={() => onViewAll(activeType)}>
            {intl.formatMessage({ id: 'notice.header.viewAll' })}
          </Button>
        </div>
      </div>
    );
  },
);

export default HeaderNoticePanel;
