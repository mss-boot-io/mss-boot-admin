import React from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { Access } from '@/components/MssBoot/Access';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import styles from '@/styles/mobile.less';

interface MobileTaskListProps {
  request: (params: any) => Promise<any>;
  onEdit: (record: any) => void;
  onCreate: () => void;
  onDelete: (record: any) => Promise<void>;
}

const MobileTaskList: React.FC<MobileTaskListProps> = ({
  request,
  onEdit,
  onCreate,
  onDelete,
}) => {
  const intl = useIntl();
  const {
    dataSource,
    error,
    hasMore,
    loading,
    loadingMore,
    loadMore,
    reload,
  } = useMobileListPagination<API.Task>(request);

  const handleDelete = async (record: any) => {
    await onDelete(record);
    await reload();
  };

  const loadMoreControl =
    error || hasMore ? (
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
            aria-label={intl.formatMessage({ id: 'pages.mobile.loadMore', defaultMessage: 'Load more' })}
          >
            {intl.formatMessage({ id: 'pages.mobile.loadMore', defaultMessage: 'Load more' })}
          </Button>
        )}
      </div>
    ) : null;

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      running: { color: 'blue', text: '运行中' },
      stopped: { color: 'default', text: '已停止' },
      success: { color: 'green', text: '成功' },
      failed: { color: 'red', text: '失败' },
    };
    const item = statusMap[status] || { color: 'default', text: status };
    return <Tag color={item.color}>{item.text}</Tag>;
  };

  return (
    <div className={styles.mobileContainer}>
      <div className={styles.toolbar}>
        <Access key="/task/create" permission="/task/create">
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建任务
          </Button>
        </Access>
      </div>

      <List
        loading={loading}
        dataSource={dataSource}
        loadMore={loadMoreControl}
        renderItem={(item) => (
          <List.Item className={styles.listItem}>
            <Card className={styles.card} size="small">
              <div className={styles.cardHeader}>
                <span className={styles.name}>{item.name}</span>
                {getStatusTag(item.status || 'stopped')}
              </div>
              
              <div className={styles.cardBody}>
                <div className={styles.field}>
                  <span className={styles.label}>提供者:</span>
                  <span className={styles.value}>{item.provider || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>命名空间:</span>
                  <span className={styles.value}>{item.namespace || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>最近检查:</span>
                  <span className={styles.value}>
                    {item.checkedAt ? new Date(item.checkedAt).toLocaleString('zh-CN', {
                      month: '2-digit',
                      day: '2-digit',
                      hour: '2-digit',
                      minute: '2-digit',
                    }) : '-'}
                  </span>
                </div>
              </div>

              <div className={styles.cardActions}>
                <Space>
                  <Access key="/task/edit" permission="/task/edit">
                    <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                  </Access>
                  <Access key="/task/delete" permission="/task/delete">
                    <Popconfirm
                      title="确定要删除吗？"
                      onConfirm={() => handleDelete(item)}
                      okText="确定"
                      cancelText="取消"
                    >
                      <Button size="small" danger icon={<DeleteOutlined />}>
                        删除
                      </Button>
                    </Popconfirm>
                  </Access>
                </Space>
              </div>
            </Card>
          </List.Item>
        )}
        locale={{ emptyText: <Empty description="暂无任务数据" /> }}
      />
    </div>
  );
};

export default MobileTaskList;
