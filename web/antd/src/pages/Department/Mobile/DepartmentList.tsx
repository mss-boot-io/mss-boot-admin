import React, { useMemo } from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { Access } from '@/components/MssBoot/Access';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import { useIntl } from '@umijs/max';
import styles from '@/styles/mobile.less';

interface MobileDepartmentListProps {
  request: (params: any) => Promise<any>;
  onEdit: (record: API.Department) => void;
  onCreate: () => void;
  onDelete: (record: API.Department) => Promise<void>;
  parentList: any[];
}

const MobileDepartmentList: React.FC<MobileDepartmentListProps> = ({
  request,
  onEdit,
  onCreate,
  onDelete,
  parentList,
}) => {
  const intl = useIntl();
  const { dataSource, error, hasMore, loading, loadingMore, loadMore, reload } =
    useMobileListPagination<API.Department>(request);

  const parentNameById = useMemo(() => {
    const names = new Map<string, string>();
    const collect = (items: any[]) => {
      items.forEach((item) => {
        if (item.value) {
          names.set(String(item.value), String(item.title || ''));
        }
        if (item.children) {
          collect(item.children);
        }
      });
    };
    collect(parentList);
    return names;
  }, [parentList]);

  const handleDelete = async (record: API.Department) => {
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

  const getParentName = (parentId: string) => {
    return parentNameById.get(parentId) || '-';
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
        <Access key="/departments/create" rootOnly>
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建部门
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
                  <span className={styles.label}>上级部门:</span>
                  <span className={styles.value}>{item.parentID ? getParentName(item.parentID) : '顶级部门'}</span>
                </div>
              </div>

              <div className={styles.cardActions}>
                <Space>
                  <Access key="/departments/edit" rootOnly>
                    <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                  </Access>
                  <Access key="/departments/delete" rootOnly>
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
        locale={{ emptyText: <Empty description="暂无部门数据" /> }}
      />
    </div>
  );
};

export default MobileDepartmentList;
