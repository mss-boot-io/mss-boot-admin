import React, { useEffect, useState } from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { Access } from '@/components/MssBoot/Access';
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
        <Access key="/task/create">
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建任务
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
                {getStatusTag(item.status || 'stopped')}
              </div>
              
              <div className={styles.cardBody}>
                <div className={styles.field}>
                  <span className={styles.label}>任务类型:</span>
                  <span className={styles.value}>{item.type || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>执行次数:</span>
                  <span className={styles.value}>{item.runCount || 0}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>最后执行:</span>
                  <span className={styles.value}>
                    {item.lastRunAt ? new Date(item.lastRunAt).toLocaleString('zh-CN', {
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
                  <Access key="/task/edit">
                    <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                  </Access>
                  <Access key="/task/delete">
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
