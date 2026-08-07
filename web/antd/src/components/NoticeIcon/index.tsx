import { message, Tag } from 'antd';
import { groupBy } from 'lodash';
import moment from 'moment';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import styles from './index.less';
import NoticeIcon from './NoticeIcon';
import { getNoticeUnread, putNoticeReadId } from '@/services/admin/notice';
import { history, useIntl } from '@umijs/max';
import { getAuthToken } from '@/utils/authStorage';

export type GlobalHeaderRightProps = {
  fetchingNotices?: boolean;
  onNoticeVisibleChange?: (visible: boolean) => void;
  onNoticeClear?: (tabName?: string) => void;
};

const getNoticeData = (notices: API.Notice[]): Record<string, API.Notice[]> => {
  if (!notices || notices.length === 0 || !Array.isArray(notices)) {
    return {};
  }

  const newNotices = notices.map((notice) => {
    const newNotice = { ...notice };

    if (newNotice.datetime) {
      newNotice.datetime = moment(notice.datetime as string).fromNow();
    }

    if (newNotice.id) {
      newNotice.key = newNotice.id;
    }

    if (newNotice.extra && newNotice.status) {
      const color = {
        todo: '',
        processing: 'blue',
        urgent: 'red',
        doing: 'gold',
      }[newNotice.status];
      newNotice.extra = (
        <Tag
          color={color}
          style={{
            marginRight: 0,
          }}
        >
          {newNotice.extra}
        </Tag>
      ) as any;
    }

    return newNotice;
  });
  return groupBy(newNotices, 'type');
};

const NOTICE_REFRESH_INTERVAL = 50000;

const NoticeIconView: React.FC = () => {
  const [notices, setNotices] = useState<API.Notice[]>([]);
  const [noticeData, setNoticeData] = useState<Record<string, API.Notice[]>>({});
  const refreshInFlightRef = useRef<Promise<void> | null>(null);
  const isMountedRef = useRef(true);
  // const { data, loading } = useRequest(getNoticeUnread);

  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const refreshNotices = useCallback((): Promise<void> => {
    if (refreshInFlightRef.current) {
      return refreshInFlightRef.current;
    }

    const refreshPromise = getNoticeUnread()
      .then((data) => {
        if (!isMountedRef.current) {
          return;
        }

        const nextNotices = data || [];
        setNotices(nextNotices);
        setNoticeData(getNoticeData(nextNotices));
      })
      .finally(() => {
        refreshInFlightRef.current = null;
      });

    refreshInFlightRef.current = refreshPromise;
    return refreshPromise;
  }, []);

  const refreshAfterMutation = useCallback(async () => {
    const inFlightRefresh = refreshInFlightRef.current;

    if (inFlightRefresh) {
      try {
        await inFlightRefresh;
      } catch {
        // The mutation still needs a fresh read after a failed background refresh.
      }
    }

    await refreshNotices();
  }, [refreshNotices]);

  const changeReadState = async (e: API.Notice) => {
    await putNoticeReadId({ id: e.id! });
    await refreshAfterMutation();
  };

  const clearReadState = async (title: string, key: string) => {
    await putNoticeReadId({ id: key });
    await refreshAfterMutation();
    message.success(
      `${intl.formatMessage({
        id: 'component.noticeIcon.cleared',
        defaultMessage: '清空了',
      })} ${title}`,
    );
  };
  useEffect(() => {
    isMountedRef.current = true;

    if (!getAuthToken()) {
      return () => {
        isMountedRef.current = false;
      };
    }

    let intervalId: ReturnType<typeof setInterval> | undefined;
    const refreshInBackground = () => {
      void refreshNotices().catch(() => undefined);
    };
    const stopPolling = () => {
      if (intervalId !== undefined) {
        clearInterval(intervalId);
        intervalId = undefined;
      }
    };
    const startPolling = () => {
      stopPolling();

      if (document.hidden) {
        return;
      }

      intervalId = setInterval(refreshInBackground, NOTICE_REFRESH_INTERVAL);
    };
    const handleVisibilityChange = () => {
      if (document.hidden) {
        stopPolling();
        return;
      }

      refreshInBackground();
      startPolling();
    };

    if (!document.hidden) {
      refreshInBackground();
      startPolling();
    }

    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      isMountedRef.current = false;
      stopPolling();
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [refreshNotices]);
  return (
    <NoticeIcon
      className={styles.action}
      count={notices.length}
      onItemClick={changeReadState}
      onClear={clearReadState}
      loading={false}
      clearText={intl.formatMessage({ id: 'component.noticeIcon.clear', defaultMessage: '清空' })}
      viewMoreText={intl.formatMessage({
        id: 'component.noticeIcon.view-more',
        defaultMessage: '查看更多',
      })}
      onViewMore={(e) => {
        // console.log(e);
        history.push(`/notice?type=${e.tabKey}`);
      }}
      clearClose
    >
      <NoticeIcon.Tab
        tabKey="notification"
        count={noticeData.notification?.length}
        list={noticeData.notification}
        title={intl.formatMessage({
          id: 'component.globalHeader.notification',
          defaultMessage: '通知',
        })}
        emptyText={intl.formatMessage({
          id: 'component.globalHeader.notification.empty',
          defaultMessage: '你已查看所有通知',
        })}
        showViewMore
      />
      <NoticeIcon.Tab
        tabKey="message"
        count={noticeData.message?.length}
        list={noticeData.message}
        title={intl.formatMessage({ id: 'component.globalHeader.message', defaultMessage: '消息' })}
        emptyText={intl.formatMessage({
          id: 'component.globalHeader.message.empty',
          defaultMessage: '您已读完所有消息',
        })}
        showViewMore
      />
      <NoticeIcon.Tab
        tabKey="event"
        title={intl.formatMessage({ id: 'component.globalHeader.event', defaultMessage: '待办' })}
        emptyText={intl.formatMessage({
          id: 'component.globalHeader.event.empty',
          defaultMessage: '你已完成所有待办',
        })}
        count={noticeData.event?.length}
        list={noticeData.event}
        showViewMore
      />
    </NoticeIcon>
  );
};

export default NoticeIconView;
