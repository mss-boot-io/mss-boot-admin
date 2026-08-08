import { Access } from '@/components/MssBoot/Access';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import styles from '@/styles/mobile.less';
import { fieldIntl } from '@/util/fieldIntl';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { Button, Card, Empty, List, message, Popconfirm, Result, Space, Tag } from 'antd';
import React from 'react';
import { useTaskOperationGuard } from '../useTaskOperationGuard';

export type TaskOperation = 'start' | 'stop';

interface MobileTaskListProps {
  request: (params: { current: number; pageSize: number }) => Promise<unknown>;
  onEdit: (record: API.Task) => void;
  onCreate: () => void;
  onDelete: (record: API.Task) => Promise<void>;
  onOperate: (record: API.Task, operation: TaskOperation) => Promise<void>;
}

export const resolveTaskOperation = (status?: API.Status): TaskOperation | undefined => {
  if (status === 'locked') {
    return undefined;
  }
  return status === 'enabled' ? 'stop' : 'start';
};

const MobileTaskList: React.FC<MobileTaskListProps> = ({
  request,
  onEdit,
  onCreate,
  onDelete,
  onOperate,
}) => {
  const intl = useIntl();
  const { pendingTaskID, runTaskOperation } = useTaskOperationGuard();
  const { dataSource, error, hasMore, loading, loadingMore, loadMore, reload } =
    useMobileListPagination<API.Task>(request);

  const handleDelete = async (record: API.Task) => {
    await onDelete(record);
    await reload();
  };

  const handleOperate = async (record: API.Task) => {
    const operation = resolveTaskOperation(record.status);
    if (!operation) {
      return;
    }

    try {
      await runTaskOperation(record.id, async () => {
        await onOperate(record, operation);
        message.success(
          intl.formatMessage({
            id: operation === 'start' ? 'pages.task.start.success' : 'pages.task.stop.success',
            defaultMessage: operation === 'start' ? 'Start successfully!' : 'Stop successfully!',
          }),
        );
        await reload();
      });
    } catch {
      // Request errors are presented by the shared request error handler.
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

  const getStatusTag = (status?: API.Status) => {
    const statusMap: Record<Exclude<API.Status, ''>, { color: string; text: string }> = {
      enabled: { color: 'green', text: fieldIntl(intl, 'options.enabled') },
      disabled: { color: 'red', text: fieldIntl(intl, 'options.disabled') },
      locked: { color: 'gold', text: fieldIntl(intl, 'options.locked') },
    };
    const item = status ? statusMap[status] : undefined;
    return <Tag color={item?.color || 'default'}>{item?.text || '-'}</Tag>;
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
      <div className={styles.toolbar}>
        <Access key="/task/create" permission="/task/create">
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            {intl.formatMessage({ id: 'pages.table.new', defaultMessage: 'New' })}
          </Button>
        </Access>
      </div>

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
            const operation = resolveTaskOperation(item.status);
            return (
              <List.Item className={styles.listItem}>
                <Card className={styles.card} size="small">
                  <div className={styles.cardHeader}>
                    <span className={styles.name}>{item.name}</span>
                    {getStatusTag(item.status)}
                  </div>

                  <div className={styles.cardBody}>
                    <div className={styles.field}>
                      <span className={styles.label}>{fieldIntl(intl, 'provider')}:</span>
                      <span className={styles.value}>{item.provider || '-'}</span>
                    </div>
                    <div className={styles.field}>
                      <span className={styles.label}>{fieldIntl(intl, 'namespace')}:</span>
                      <span className={styles.value}>{item.namespace || '-'}</span>
                    </div>
                    <div className={styles.field}>
                      <span className={styles.label}>{fieldIntl(intl, 'checkedAt')}:</span>
                      <span className={styles.value}>{formatDateTime(item.checkedAt)}</span>
                    </div>
                  </div>

                  <div className={styles.cardActions}>
                    <Space wrap>
                      {operation ? (
                        <Access key="/task/operate" permission="/task/operate">
                          <Button
                            size="small"
                            loading={pendingTaskID === item.id}
                            disabled={Boolean(pendingTaskID && pendingTaskID !== item.id)}
                            onClick={() => handleOperate(item)}
                          >
                            {intl.formatMessage({
                              id:
                                operation === 'start'
                                  ? 'pages.task.start.title'
                                  : 'pages.task.stop.title',
                              defaultMessage: operation === 'start' ? 'Start' : 'Stop',
                            })}
                          </Button>
                        </Access>
                      ) : null}
                      <Access key="/task/edit" permission="/task/edit">
                        <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                          {intl.formatMessage({ id: 'pages.title.edit', defaultMessage: 'Edit' })}
                        </Button>
                      </Access>
                      <Access key="/task/delete" permission="/task/delete">
                        <Popconfirm
                          title={intl.formatMessage({
                            id: 'pages.description.delete.confirm',
                            defaultMessage: 'Are you sure to delete this record?',
                          })}
                          onConfirm={() => handleDelete(item)}
                          okText={intl.formatMessage({
                            id: 'pages.title.ok',
                            defaultMessage: 'OK',
                          })}
                          cancelText={intl.formatMessage({
                            id: 'pages.title.cancel',
                            defaultMessage: 'Cancel',
                          })}
                        >
                          <Button size="small" danger icon={<DeleteOutlined />}>
                            {intl.formatMessage({
                              id: 'pages.title.delete',
                              defaultMessage: 'Delete',
                            })}
                          </Button>
                        </Popconfirm>
                      </Access>
                    </Space>
                  </div>
                </Card>
              </List.Item>
            );
          }}
          locale={{ emptyText: <Empty /> }}
        />
      )}
    </div>
  );
};

export default MobileTaskList;
