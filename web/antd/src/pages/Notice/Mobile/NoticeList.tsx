import React, { useEffect, useState } from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { Access } from '@/components/MssBoot/Access';
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
  const [loading, setLoading] = useState(false);
  const [dataSource, setDataSource] = useState<any[]>([]);

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

  const handleDelete = async (record: any) => {
    await onDelete(record);
    fetchData();
  };

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
        <Access key="/notice/create">
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建通知
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
                <span className={styles.name}>{item.title}</span>
                {getTypeTag(item.type || 'notice')}
              </div>
              
              <div className={styles.cardBody}>
                <div className={styles.field}>
                  <span className={styles.label}>内容:</span>
                  <span className={styles.value}>{(item.content || '-').substring(0, 50)}...</span>
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
                  <Access key="/notice/edit">
                    <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                  </Access>
                  <Access key="/notice/delete">
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
