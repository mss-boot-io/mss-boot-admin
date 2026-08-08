import React from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { Access } from '@/components/MssBoot/Access';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import { useIntl } from '@umijs/max';
import styles from '@/styles/mobile.less';

interface MobilePostListProps {
  request: (params: any) => Promise<any>;
  onEdit: (record: API.Post) => void;
  onCreate: () => void;
  onDelete: (record: API.Post) => Promise<void>;
  dataScopeValueEnum: any;
}

const MobilePostList: React.FC<MobilePostListProps> = ({
  request,
  onEdit,
  onCreate,
  onDelete,
  dataScopeValueEnum,
}) => {
  const intl = useIntl();
  const { dataSource, error, hasMore, loading, loadingMore, loadMore, reload } =
    useMobileListPagination<API.Post>(request);

  const handleDelete = async (record: API.Post) => {
    await onDelete(record);
    await reload();
  };

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      enabled: { color: 'green', text: '启用' },
      disabled: { color: 'red', text: '禁用' },
    };
    const item = statusMap[status] || { color: 'default', text: status };
    return <Tag color={item.color}>{item.text}</Tag>;
  };

  const getDataScopeLabel = (dataScope: string) => {
    if (dataScopeValueEnum && dataScopeValueEnum[dataScope]) {
      return dataScopeValueEnum[dataScope].text;
    }
    return dataScope || '-';
  };

  const loadMoreControl =
    error || hasMore ? (
      <div style={{ padding: '12px 0', textAlign: 'center' }}>
        <Button onClick={error ? reload : loadMore} loading={loadingMore}>
          {error
            ? intl.formatMessage({ id: 'pages.mobile.retry', defaultMessage: 'Retry' })
            : intl.formatMessage({ id: 'pages.mobile.loadMore', defaultMessage: 'Load more' })}
        </Button>
      </div>
    ) : null;

  return (
    <div className={styles.mobileContainer}>
      <div className={styles.toolbar}>
        <Access key="/posts/create" rootOnly>
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建岗位
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
                  <span className={styles.label}>编码:</span>
                  <span className={styles.value}>{item.code || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>数据权限:</span>
                  <span className={styles.value}>{getDataScopeLabel(item.dataScope || '')}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>排序:</span>
                  <span className={styles.value}>{item.sort || '-'}</span>
                </div>
              </div>

              <div className={styles.cardActions}>
                <Space>
                  <Access key="/posts/edit" rootOnly>
                    <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                  </Access>
                  <Access key="/posts/delete" rootOnly>
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
        locale={{ emptyText: <Empty description="暂无岗位数据" /> }}
      />
    </div>
  );
};

export default MobilePostList;
