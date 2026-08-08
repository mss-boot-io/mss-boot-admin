import { Access } from '@/components/MssBoot/Access';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import styles from '@/styles/mobile.less';
import { fieldIntl } from '@/util/fieldIntl';
import { Button, Card, Empty, List, message, Result, Space, Tag } from 'antd';
import React, { useRef, useState } from 'react';
import { useIntl } from '@umijs/max';

interface MobileNoticeListProps {
  request: (params: { current: number; pageSize: number }) => Promise<unknown>;
  onView: (record: API.Notice) => void;
  onMarkRead: (record: API.Notice) => Promise<void>;
}

const MobileNoticeList: React.FC<MobileNoticeListProps> = ({ request, onView, onMarkRead }) => {
  const intl = useIntl();
  const markingReadIDRef = useRef<string>();
  const [markingReadID, setMarkingReadID] = useState<string>();
  const { dataSource, error, hasMore, loading, loadingMore, loadMore, reload } =
    useMobileListPagination<API.Notice>(request);

  const handleMarkRead = async (record: API.Notice) => {
    if (!record.id || markingReadIDRef.current) {
      return;
    }

    markingReadIDRef.current = record.id;
    setMarkingReadID(record.id);
    try {
      await onMarkRead(record);
      message.success(
        intl.formatMessage({
          id: 'pages.title.notice.read',
          defaultMessage: 'Mark as read',
        }),
      );
      await reload();
    } catch {
      // Request errors are presented by the shared request error handler.
    } finally {
      markingReadIDRef.current = undefined;
      setMarkingReadID(undefined);
    }
  };

  const initialLoadError = Boolean(error && dataSource.length === 0);
  const loadMoreControl =
    !initialLoadError && (error || hasMore) ? (
      <div style={{ margin: '16px 0', textAlign: 'center' }} role="status" aria-live="polite">
        {error ? (
          <Button
            type="link"
            onClick={reload}
            aria-label={intl.formatMessage({ id: 'pages.mobile.retry', defaultMessage: 'Retry' })}
          >
            {intl.formatMessage({ id: 'pages.mobile.retry', defaultMessage: 'Retry' })}
          </Button>
        ) : (
          <Button
            type="link"
            loading={loadingMore}
            onClick={loadMore}
            aria-label={intl.formatMessage({
              id: 'pages.mobile.loadMore',
              defaultMessage: 'Load more',
            })}
          >
            {intl.formatMessage({ id: 'pages.mobile.loadMore', defaultMessage: 'Load more' })}
          </Button>
        )}
      </div>
    ) : null;

  const getTypeTag = (type?: API.NoticeType) => {
    const typeMap: Record<API.NoticeType, { color: string; text: string }> = {
      notification: { color: 'red', text: fieldIntl(intl, 'options.notification') },
      message: { color: 'blue', text: fieldIntl(intl, 'options.message') },
      event: { color: 'gold', text: fieldIntl(intl, 'options.event') },
      mail: { color: 'cyan', text: fieldIntl(intl, 'options.email') },
    };
    const item = type ? typeMap[type] : undefined;
    return <Tag color={item?.color || 'default'}>{item?.text || '-'}</Tag>;
  };

  const getStatusTag = (status?: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      urgent: { color: 'red', text: fieldIntl(intl, 'options.urgent') },
      doing: { color: 'green', text: fieldIntl(intl, 'options.doing') },
      processing: { color: 'blue', text: fieldIntl(intl, 'options.processing') },
      todo: { color: 'gold', text: fieldIntl(intl, 'options.todo') },
    };
    const item = status ? statusMap[status] : undefined;
    return <Tag color={item?.color || 'default'}>{item?.text || status || '-'}</Tag>;
  };

  const formatDateTime = (value?: string) =>
    value
      ? intl.formatDate(value, {
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
        })
      : '-';

  return (
    <div className={styles.mobileContainer}>
      {initialLoadError ? (
        <Result
          status="error"
          title={intl.formatMessage({
            id: 'pages.mobile.loadFailed',
            defaultMessage: 'Unable to load data',
          })}
          extra={
            <Button type="primary" onClick={reload}>
              {intl.formatMessage({ id: 'pages.mobile.retry', defaultMessage: 'Retry' })}
            </Button>
          }
        />
      ) : (
        <List
          loading={loading}
          dataSource={dataSource}
          loadMore={loadMoreControl}
          renderItem={(item) => {
            const description = item.description || '-';
            const summarizedDescription =
              description.length > 50 ? `${description.substring(0, 50)}...` : description;

            return (
              <List.Item className={styles.listItem}>
                <Card className={styles.card} size="small">
                  <div className={styles.cardHeader}>
                    <Button type="link" size="small" onClick={() => onView(item)}>
                      {item.title || '-'}
                    </Button>
                    <Space size={4} wrap>
                      {getTypeTag(item.type)}
                      {getStatusTag(item.status)}
                    </Space>
                  </div>

                  <div className={styles.cardBody}>
                    <div className={styles.field}>
                      <span className={styles.label}>{fieldIntl(intl, 'description')}:</span>
                      <span className={styles.value}>{summarizedDescription}</span>
                    </div>
                    <div className={styles.field}>
                      <span className={styles.label}>{fieldIntl(intl, 'createdAt')}:</span>
                      <span className={styles.value}>{formatDateTime(item.createdAt)}</span>
                    </div>
                  </div>

                  {!item.read && item.id ? (
                    <div className={styles.cardActions}>
                      <Access key="/notice/read" permission="/notice/read">
                        <Button
                          size="small"
                          loading={markingReadID === item.id}
                          disabled={Boolean(markingReadID && markingReadID !== item.id)}
                          onClick={() => handleMarkRead(item)}
                        >
                          {intl.formatMessage({
                            id: 'pages.title.notice.read',
                            defaultMessage: 'Mark as read',
                          })}
                        </Button>
                      </Access>
                    </div>
                  ) : null}
                </Card>
              </List.Item>
            );
          }}
          locale={{
            emptyText: (
              <Empty
                description={intl.formatMessage({
                  id: 'component.noticeIcon.empty',
                  defaultMessage: 'No notifications',
                })}
              />
            ),
          }}
        />
      )}
    </div>
  );
};

export default MobileNoticeList;
