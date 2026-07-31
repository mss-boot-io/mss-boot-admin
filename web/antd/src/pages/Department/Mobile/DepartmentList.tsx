import React, { useEffect, useState } from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { Access } from '@/components/MssBoot/Access';
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
  const [loading, setLoading] = useState(false);
  const [dataSource, setDataSource] = useState<API.Department[]>([]);

  const fetchData = async () => {
    setLoading(true);
    try {
      const response = await request({ pageSize: 100 });
      setDataSource(response?.data || []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleDelete = async (record: API.Department) => {
    await onDelete(record);
    fetchData();
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
    const findParent = (items: any[], id: string): string => {
      for (const item of items) {
        if (item.value === id) return item.title;
        if (item.children) {
          const found = findParent(item.children, id);
          if (found) return found;
        }
      }
      return '-';
    };
    return findParent(parentList, parentId);
  };

  return (
    <div className={styles.mobileContainer}>
      <div className={styles.toolbar}>
        <Access key="/departments/create">
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建部门
          </Button>
        </Access>
      </div>

      <List
        loading={loading}
        dataSource={dataSource}
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
                  <Access key="/departments/edit">
                    <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                  </Access>
                  <Access key="/departments/delete">
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
