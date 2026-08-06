import React from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { Access } from '@/components/MssBoot/Access';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import styles from '@/styles/mobile.less';

interface MobileNoticeListProps {
  request: (params: any) => Promise<any>;
  onEdit: (record: any) => void;
  onCreate: () => void;
  onDelete: (record: any) => Promise<void>;
}

const MobileNoticeList: React.FC<MobileNoticeListProps> = ({
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
  } = useMobileListPagination<API.Notice>(request);

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

  const getTypeTag = (type: string) => {
    const typeMap: Record<string, { color: string; text: string }> = {
      announcement: { color: 'blue', text: '公告' },
      notice: { color: 'green', text: '通知' },
      warning: { color: 'orange', text: '警告' },
    };
    const item = typeMap[type] || { color: 'default', text: type };
    return <Tag color={item.color}>{item.text}</Tag>;
  };

  return (
    <div className={styles.mobileContainer}>
      <div className={styles.toolbar}>
        <Access key="/notice/create" permission="/notice/create">
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建通知
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
                <span className={styles.name}>{item.title}</span>
                {getTypeTag(item.type || 'notice')}
              </div>
              
              <div className={styles.cardBody}>
                <div className={styles.field}>
                  <span className={styles.label}>内容:</span>
                  <span className={styles.value}>{(item.description || '-').substring(0, 50)}...</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>创建时间:</span>
                  <span className={styles.value}>
                    {item.createdAt ? new Date(item.createdAt).toLocaleString('zh-CN', {
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
                  <Access key="/notice/edit" permission="/notice/edit">
                    <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                  </Access>
                  <Access key="/notice/delete" permission="/notice/delete">
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
        locale={{ emptyText: <Empty description="暂无通知数据" /> }}
      />
    </div>
  );
};

export default MobileNoticeList;
