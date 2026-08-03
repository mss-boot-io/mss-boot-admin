import React, { useEffect, useState } from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { Access } from '@/components/MssBoot/Access';
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
  const [loading, setLoading] = useState(false);
  const [dataSource, setDataSource] = useState<API.Post[]>([]);

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

  const handleDelete = async (record: API.Post) => {
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

  const getDataScopeLabel = (dataScope: string) => {
    if (dataScopeValueEnum && dataScopeValueEnum[dataScope]) {
      return dataScopeValueEnum[dataScope].text;
    }
    return dataScope || '-';
  };

  return (
    <div className={styles.mobileContainer}>
      <div className={styles.toolbar}>
        <Access key="/posts/create">
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建岗位
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
                  <Access key="/posts/edit">
                    <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                  </Access>
                  <Access key="/posts/delete">
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
