import { deleteUsersId, getUsers } from '@/services/admin/user';
import { Avatar, Card, List, Tag, Button, Space, Popconfirm } from 'antd';
import { DeleteOutlined, EditOutlined, UserOutlined } from '@ant-design/icons';
import { useRequest, history } from '@umijs/max';
import React from 'react';
import styles from '@/styles/mobile.less';

const MobileUserList: React.FC = () => {
  const { data, loading, refresh } = useRequest(() => getUsers({ pageSize: 100 }));
  const userList = Array.isArray(data) ? data : [];

  const handleEdit = (id: string) => {
    history.push(`/user/edit/${id}`);
  };

  const handleDelete = async (id: string) => {
    await deleteUsersId({ id });
    refresh();
  };

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      enabled: { color: 'green', text: '启用' },
      disabled: { color: 'red', text: '禁用' },
      locked: { color: 'orange', text: '锁定' },
    };
    const config = statusMap[status] || { color: 'default', text: status };
    return <Tag color={config.color}>{config.text}</Tag>;
  };

  return (
    <div className={styles.mobileContainer}>
      <List
        loading={loading}
        dataSource={userList}
        renderItem={(item: API.User) => (
          <List.Item className={styles.listItem}>
            <Card className={styles.card} size="small">
              <div className={styles.cardHeader}>
                <Space>
                  <Avatar icon={<UserOutlined />} src={item.avatar} />
                  <span className={styles.name}>{item.name || item.username}</span>
                </Space>
                {getStatusTag(item.status || 'enabled')}
              </div>
              
              <div className={styles.cardBody}>
                <div className={styles.field}>
                  <span className={styles.label}>邮箱:</span>
                  <span className={styles.value}>{item.email || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>手机:</span>
                  <span className={styles.value}>{item.phone || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>部门:</span>
                  <span className={styles.value}>{item.department?.name || '-'}</span>
                </div>
              </div>

              <div className={styles.cardActions}>
                <Space>
                  <Button
                    type="text"
                    icon={<EditOutlined />}
                    onClick={() => handleEdit(item.id || '')}
                    size="small"
                  >
                    编辑
                  </Button>
                  <Popconfirm title="确定要删除吗？" onConfirm={() => handleDelete(item.id || '')}>
                    <Button type="text" danger icon={<DeleteOutlined />} size="small">
                      删除
                    </Button>
                  </Popconfirm>
                </Space>
              </div>
            </Card>
          </List.Item>
        )}
        locale={{ emptyText: '暂无用户数据' }}
      />
    </div>
  );
};

export default MobileUserList;