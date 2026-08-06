import React from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { Access } from '@/components/MssBoot/Access';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import styles from '@/styles/mobile.less';

interface MobileLanguageListProps {
  request: (params: any) => Promise<any>;
  onEdit: (record: any) => void;
  onCreate: () => void;
  onDelete: (record: any) => Promise<void>;
}

const MobileLanguageList: React.FC<MobileLanguageListProps> = ({
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
  } = useMobileListPagination<API.Language>(request);

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
      enabled: { color: 'green', text: '启用' },
      disabled: { color: 'red', text: '禁用' },
    };
    const item = statusMap[status] || { color: 'default', text: status };
    return <Tag color={item.color}>{item.text}</Tag>;
  };

  return (
    <div className={styles.mobileContainer}>
      <div className={styles.toolbar}>
        <Access key="/language/create" permission="/language/create">
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建语言
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
                {getStatusTag(item.status || 'enabled')}
              </div>
              
              <div className={styles.cardBody}>
                <div className={styles.field}>
                  <span className={styles.label}>定义数:</span>
                  <span className={styles.value}>{item.defines?.length || 0}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>备注:</span>
                  <span className={styles.value}>{item.remark || '-'}</span>
                </div>
              </div>

              <div className={styles.cardActions}>
                <Space>
                  <Access key="/language/edit" permission="/language/edit">
                    <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                  </Access>
                  <Access key="/language/delete" permission="/language/delete">
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
        locale={{ emptyText: <Empty description="暂无语言数据" /> }}
      />
    </div>
  );
};

export default MobileLanguageList;
