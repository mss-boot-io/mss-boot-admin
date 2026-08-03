import React, { useEffect, useState } from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { Access } from '@/components/MssBoot/Access';
import styles from '@/styles/mobile.less';

interface MobileOptionListProps {
  request: (params: any) => Promise<any>;
  onEdit: (record: API.Option) => void;
  onCreate: () => void;
  onDelete: (record: API.Option) => Promise<void>;
}

const MobileOptionList: React.FC<MobileOptionListProps> = ({
  request,
  onEdit,
  onCreate,
  onDelete,
}) => {
  const [loading, setLoading] = useState(false);
  const [dataSource, setDataSource] = useState<API.Option[]>([]);

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

  const handleDelete = async (record: API.Option) => {
    await onDelete(record);
    fetchData();
  };

  return (
    <div className={styles.mobileContainer}>
      <div className={styles.toolbar}>
        <Access key="/option/create">
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建选项
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
                <Tag color="blue">{item.category || '-'}</Tag>
              </div>
              
              <div className={styles.cardBody}>
                <div className={styles.field}>
                  <span className={styles.label}>显示名称:</span>
                  <span className={styles.value}>{item.displayName || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>描述:</span>
                  <span className={styles.value}>{item.description || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>版本:</span>
                  <span className={styles.value}>{item.version || 1}</span>
                </div>
              </div>

              <div className={styles.cardActions}>
                <Space>
                  <Access key="/option/edit">
                    <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                  </Access>
                  <Access key="/option/delete">
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
        locale={{ emptyText: <Empty description="暂无选项数据" /> }}
      />
    </div>
  );
};

export default MobileOptionList;
