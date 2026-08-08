import { BellOutlined } from '@ant-design/icons';
import { Badge, Button, Result, Spin, Tabs } from 'antd';
import classNames from 'classnames';
import useMergedState from 'rc-util/es/hooks/useMergedState';
import React, { useEffect, useRef } from 'react';
import HeaderDropdown from '../HeaderDropdown';
import styles from './index.less';
import type { NoticeIconTabProps } from './NoticeList';
import NoticeList from './NoticeList';
import type { TabsProps } from 'antd';

const NOTICE_FOCUSABLE_SELECTOR =
  '[role="tab"][tabindex="0"], button:not([disabled]):not([tabindex="-1"]):not([aria-hidden="true"]), a[href], [tabindex]:not([tabindex="-1"]):not([aria-hidden="true"]):not([role="tabpanel"])';

const getNoticeFocusableElements = (dialog: HTMLDivElement | null): HTMLElement[] =>
  dialog ? Array.from(dialog.querySelectorAll<HTMLElement>(NOTICE_FOCUSABLE_SELECTOR)) : [];

export type NoticeIconProps = {
  ariaLabel?: string;
  count?: number;
  bell?: React.ReactNode;
  className?: string;
  loading?: boolean;
  loadingText?: string;
  errorText?: string;
  retryText?: string;
  onRetry?: () => void;
  onClear?: (tabName: string, tabKey: string) => void;
  onItemClick?: (item: API.Notice, tabProps: NoticeIconTabProps) => void;
  onViewMore?: (tabProps: NoticeIconTabProps, e: MouseEvent) => void;
  onTabChange?: (tabTile: string) => void;
  style?: React.CSSProperties;
  onPopupVisibleChange?: (visible: boolean) => void;
  popupVisible?: boolean;
  clearText?: string;
  viewMoreText?: string;
  clearClose?: boolean;
  children?: React.ReactElement<NoticeIconTabProps>[];
};

const NoticeIcon: React.FC<NoticeIconProps> & {
  Tab: typeof NoticeList;
} = (props) => {
  const [visible, setVisible] = useMergedState<boolean>(false, {
    value: props.popupVisible,
    onChange: props.onPopupVisibleChange,
  });
  const dialogRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const wasVisibleRef = useRef(false);

  useEffect(() => {
    if (visible) {
      const animationFrame = window.requestAnimationFrame(() => {
        const [firstFocusable] = getNoticeFocusableElements(dialogRef.current);
        (firstFocusable || dialogRef.current)?.focus();
      });
      wasVisibleRef.current = true;
      return () => window.cancelAnimationFrame(animationFrame);
    }

    if (wasVisibleRef.current) {
      triggerRef.current?.focus();
      wasVisibleRef.current = false;
    }

    return undefined;
  }, [visible]);

  const getNotificationBox = (): React.ReactNode => {
    const {
      children,
      loading,
      onClear,
      onTabChange,
      onItemClick,
      onViewMore,
      clearText,
      viewMoreText,
      ariaLabel,
      loadingText,
      errorText,
      retryText,
      onRetry,
    } = props;
    if (!children) {
      return null;
    }
    const items: TabsProps['items'] = [];
    React.Children.forEach(children, (child: React.ReactElement<NoticeIconTabProps>): void => {
      if (!child) {
        return;
      }

      const { list, title, count, tabKey, showClear, showViewMore } = child.props;
      const len = list && list.length ? list.length : 0;
      const msgCount = count || count === 0 ? count : len;
      const tabTitle: string = msgCount > 0 ? `${title} (${msgCount})` : title;

      items.push({
        key: tabKey,
        label: tabTitle,
        children: (
          <NoticeList
            clearText={clearText}
            viewMoreText={viewMoreText}
            list={list}
            tabKey={tabKey}
            onClear={(): void => onClear && onClear(title, tabKey)}
            onClick={(item): void => onItemClick && onItemClick(item, child.props)}
            onViewMore={(event): void => {
              setVisible(false);
              onViewMore?.(child.props, event);
            }}
            showClear={showClear}
            showViewMore={showViewMore}
            title={title}
          />
        ),
      });
    });
    const content = loading ? (
      <div className={styles.state} role="status" aria-live="polite">
        <Spin size="small" />
        <span>{loadingText}</span>
      </div>
    ) : errorText ? (
      <div role="alert" aria-live="assertive">
        <Result
          status="error"
          title={errorText}
          extra={
            <Button type="primary" size="small" onClick={onRetry}>
              {retryText}
            </Button>
          }
        />
      </div>
    ) : (
      <Tabs className={styles.tabs} onChange={onTabChange} items={items} />
    );

    return (
      <div
        ref={dialogRef}
        role="dialog"
        aria-label={ariaLabel}
        tabIndex={-1}
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            event.preventDefault();
            setVisible(false);
            return;
          }

          if (event.key === 'Tab') {
            event.preventDefault();
            event.stopPropagation();
            const focusableElements = getNoticeFocusableElements(dialogRef.current);
            if (focusableElements.length === 0) {
              dialogRef.current?.focus();
              return;
            }

            const currentIndex = focusableElements.indexOf(document.activeElement as HTMLElement);
            let nextIndex: number;
            if (event.shiftKey) {
              nextIndex = currentIndex <= 0 ? focusableElements.length - 1 : currentIndex - 1;
            } else {
              nextIndex =
                currentIndex < 0 || currentIndex === focusableElements.length - 1
                  ? 0
                  : currentIndex + 1;
            }
            focusableElements[nextIndex]?.focus();
          }
        }}
      >
        {content}
      </div>
    );
  };

  const { ariaLabel, className, count, bell } = props;

  const noticeButtonClass = classNames(className, styles.noticeButton);
  const notificationBox = getNotificationBox();
  const NoticeBellIcon = bell || <BellOutlined className={styles.icon} />;
  const trigger = (
    <button
      ref={triggerRef}
      type="button"
      className={classNames(noticeButtonClass, { opened: visible })}
      aria-label={ariaLabel}
      aria-haspopup="dialog"
      aria-expanded={visible}
    >
      <Badge count={count} style={{ boxShadow: 'none' }} className={styles.badge}>
        {NoticeBellIcon}
      </Badge>
    </button>
  );
  if (!notificationBox) {
    return trigger;
  }

  return (
    <HeaderDropdown
      placement="bottomRight"
      // overlay={notificationBox}
      dropdownRender={getNotificationBox}
      overlayClassName={styles.popover}
      trigger={['click']}
      open={visible}
      // @ts-ignore
      onOpenChange={setVisible}
    >
      {trigger}
    </HeaderDropdown>
  );
};

NoticeIcon.Tab = NoticeList;

export default NoticeIcon;
