import React from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, SafetyOutlined } from '@ant-design/icons';
import { Access } from '@/components/MssBoot/Access';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import { useIntl } from '@umijs/max';
import styles from '@/styles/mobile.less';
import { getRoleActionDisabledState } from '../roleActions';

interface MobileRoleListProps {
  request: (params: any) => Promise<API.Page & { data?: API.Role[] }>;
  onEdit: (record: API.Role) => void;
  onCreate: () => void;
  onAuth: (record: API.Role) => void;
  onDelete: (record: API.Role) => Promise<void>;
}

const MobileRoleList: React.FC<MobileRoleListProps> = ({
  request,
  onEdit,
  onCreate,
  onAuth,
  onDelete,
}) => {
  const intl = useIntl();
  const { dataSource, error, hasMore, loading, loadingMore, loadMore, reload } =
    useMobileListPagination<API.Role>(request);

  const handleDelete = async (record: API.Role) => {
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
        <Access key="/role/create" rootOnly>
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建角色
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
                {item.root && <Tag color="gold">超级角色</Tag>}
                {getStatusTag(item.status || 'enabled')}
              </div>
              
              <div className={styles.cardBody}>
                <div className={styles.field}>
                  <span className={styles.label}>ID:</span>
                  <span className={styles.value}>{item.id?.substring(0, 8)}...</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>备注:</span>
                  <span className={styles.value}>{item.remark || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>更新时间:</span>
                  <span className={styles.value}>
                    {item.updatedAt ? new Date(item.updatedAt).toLocaleString('zh-CN', {
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
                  <Access key="/role/edit" rootOnly>
                    <Button
                      size="small"
                      icon={<EditOutlined />}
                      disabled={getRoleActionDisabledState(item).edit}
                      onClick={() => onEdit(item)}
                    >
                      编辑
                    </Button>
                  </Access>
                  <Access key="/role/auth" rootOnly>
                    <Button
                      size="small"
                      icon={<SafetyOutlined />}
                      disabled={getRoleActionDisabledState(item).authorize}
                      onClick={() => onAuth(item)}
                    >
                      授权
                    </Button>
                  </Access>
                  <Access key="/role/delete" rootOnly>
                    <Popconfirm
                      title="确定要删除吗？"
                      onConfirm={() => handleDelete(item)}
                      okText="确定"
                      cancelText="取消"
                      disabled={getRoleActionDisabledState(item).delete}
                    >
                      <Button
                        size="small"
                        danger
                        icon={<DeleteOutlined />}
                        disabled={getRoleActionDisabledState(item).delete}
                      >
                        删除
                      </Button>
                    </Popconfirm>
                  </Access>
                </Space>
              </div>
            </Card>
          </List.Item>
        )}
        locale={{ emptyText: <Empty description="暂无角色数据" /> }}
      />
    </div>
  );
};

export default MobileRoleList;
