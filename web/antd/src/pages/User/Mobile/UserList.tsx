import { Access } from '@/components/MssBoot/Access';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import { deleteUsersId, getUsers } from '@/services/admin/user';
import { DeleteOutlined, EditOutlined, PlusOutlined, UserOutlined } from '@ant-design/icons';
import { history, useIntl } from '@umijs/max';
import { Avatar, Button, Card, Empty, Input, List, Popconfirm, Space, Tag } from 'antd';
import React, { useCallback, useState } from 'react';
import styles from '@/styles/mobile.less';

const MobileUserList: React.FC = () => {
  const intl = useIntl();
  const [searchInput, setSearchInput] = useState('');
  const [name, setName] = useState('');
  const requestUsers = useCallback(
    ({ current, pageSize }: { current: number; pageSize: number }) =>
      getUsers({ current, pageSize, ...(name ? { name } : {}) }),
    [name],
  );
  const { dataSource: userList, error, hasMore, loading, loadingMore, loadMore, reload } =
    useMobileListPagination<API.User>(requestUsers);

  const handleEdit = (id?: string) => {
    if (id) {
      history.push(`/users/control/${id}`);
    }
  };

  const handleCreate = () => {
    history.push('/users/control/create');
  };

  const handleDelete = async (id?: string) => {
    if (!id) {
      return;
    }
    await deleteUsersId({ id });
    await reload();
  };

  const handleSearch = (value: string) => {
    setSearchInput(value);
    setName(value.trim());
  };

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      enabled: {
        color: 'green',
        text: intl.formatMessage({ id: 'pages.fields.options.enabled', defaultMessage: 'Enabled' }),
      },
      disabled: {
        color: 'red',
        text: intl.formatMessage({ id: 'pages.fields.options.disabled', defaultMessage: 'Disabled' }),
      },
      locked: {
        color: 'orange',
        text: intl.formatMessage({ id: 'pages.fields.options.locked', defaultMessage: 'Locked' }),
      },
    };
    const config = statusMap[status] || { color: 'default', text: status };
    return <Tag color={config.color}>{config.text}</Tag>;
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
        <div style={{ display: 'flex', gap: 8 }}>
          <Input.Search
            allowClear
            value={searchInput}
            onChange={(event) => {
              const nextValue = event.target.value;
              setSearchInput(nextValue);
              if (!nextValue) {
                setName('');
              }
            }}
            onSearch={handleSearch}
            placeholder={intl.formatMessage({
              id: 'pages.user.mobile.search.placeholder',
              defaultMessage: 'Search users by name',
            })}
            style={{ flex: '1 1 auto', minWidth: 0 }}
          />
          <Access key="/users/create" rootOnly>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
              {intl.formatMessage({ id: 'pages.user.mobile.create', defaultMessage: 'New User' })}
            </Button>
          </Access>
        </div>
      </div>

      <List
        loading={loading}
        dataSource={userList}
        loadMore={loadMoreControl}
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
                  <span className={styles.label}>
                    {intl.formatMessage({ id: 'pages.fields.email', defaultMessage: 'Email' })}:
                  </span>
                  <span className={styles.value}>{item.email || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>
                    {intl.formatMessage({ id: 'pages.fields.phone', defaultMessage: 'Phone' })}:
                  </span>
                  <span className={styles.value}>{item.phone || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>
                    {intl.formatMessage({ id: 'pages.fields.department', defaultMessage: 'Department' })}:
                  </span>
                  <span className={styles.value}>{item.department?.name || '-'}</span>
                </div>
              </div>

              <div className={styles.cardActions}>
                <Space>
                  <Access key="/users/edit" rootOnly>
                    <Button
                      type="text"
                      icon={<EditOutlined />}
                      onClick={() => handleEdit(item.id)}
                      size="small"
                      disabled={!item.id}
                    >
                      {intl.formatMessage({ id: 'pages.title.edit', defaultMessage: 'Edit' })}
                    </Button>
                  </Access>
                  <Access key="/users/delete" rootOnly>
                    <Popconfirm
                      title={intl.formatMessage({
                        id: 'pages.title.delete.confirm',
                        defaultMessage: 'Confirm Delete',
                      })}
                      description={intl.formatMessage({
                        id: 'pages.description.delete.confirm',
                        defaultMessage: 'Are you sure to delete this record?',
                      })}
                      onConfirm={() => handleDelete(item.id)}
                      okText={intl.formatMessage({ id: 'pages.title.ok', defaultMessage: 'OK' })}
                      cancelText={intl.formatMessage({ id: 'pages.title.cancel', defaultMessage: 'Cancel' })}
                      disabled={!item.id}
                    >
                      <Button type="text" danger icon={<DeleteOutlined />} size="small" disabled={!item.id}>
                        {intl.formatMessage({ id: 'pages.title.delete', defaultMessage: 'Delete' })}
                      </Button>
                    </Popconfirm>
                  </Access>
                </Space>
              </div>
            </Card>
          </List.Item>
        )}
        locale={{
          emptyText: (
            <Empty
              description={intl.formatMessage({
                id: 'pages.user.mobile.empty',
                defaultMessage: 'No users found',
              })}
            />
          ),
        }}
      />
    </div>
  );
};

export default MobileUserList;
