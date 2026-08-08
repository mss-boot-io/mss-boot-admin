import React from 'react';
import { Button, Card, Empty, List, Space, Tag, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { Access } from '@/components/MssBoot/Access';
import { useMobileListPagination } from '@/hooks/useMobileListPagination';
import styles from '@/styles/mobile.less';
import { getMenuLocaleId } from '../menuLocale';

interface MobileMenuListProps {
  request: (params: any) => Promise<any>;
  onEdit: (record: API.Menu) => void;
  onCreate: () => void;
  onDelete: (record: API.Menu) => Promise<void>;
}

const MobileMenuList: React.FC<MobileMenuListProps> = ({
  request,
  onEdit,
  onCreate,
  onDelete,
}) => {
  const intl = useIntl();
  const {
    dataSource,
    error,
    hasMore,
    loading,
    loadingMore,
    loadMore,
    reload,
  } = useMobileListPagination<API.Menu>(request);

  const handleDelete = async (record: API.Menu) => {
    await onDelete(record);
    await reload();
  };

  const loadMoreControl =
    error || hasMore ? (
      <div style={{ margin: '16px 0', textAlign: 'center' }} role="status" aria-live="polite">
        {error ? (
          <Button
            type="link"
            onClick={reload}
            aria-label={intl.formatMessage({ id: 'pages.mobile.retry', defaultMessage: 'Retry' })}
          >
            {intl.formatMessage({ id: 'pages.mobile.retry', defaultMessage: 'Retry' })}
          </Button>
        ) : (
          <Button
            type="link"
            loading={loadingMore}
            onClick={loadMore}
            aria-label={intl.formatMessage({ id: 'pages.mobile.loadMore', defaultMessage: 'Load more' })}
          >
            {intl.formatMessage({ id: 'pages.mobile.loadMore', defaultMessage: 'Load more' })}
          </Button>
        )}
      </div>
    ) : null;

  const getTypeTag = (type: string) => {
    const typeMap: Record<string, { color: string; text: string }> = {
      DIRECTORY: { color: 'blue', text: '目录' },
      MENU: { color: 'green', text: '菜单' },
      COMPONENT: { color: 'orange', text: '组件' },
      API: { color: 'purple', text: 'API' },
    };
    const item = typeMap[type] || { color: 'default', text: type };
    return <Tag color={item.color}>{item.text}</Tag>;
  };

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      enabled: { color: 'green', text: '启用' },
      disabled: { color: 'red', text: '禁用' },
    };
    const item = statusMap[status] || { color: 'default', text: status };
    return <Tag color={item.color}>{item.text}</Tag>;
  };

  const localizeMenuName = (name?: string) => {
    if (!name) {
      return '';
    }

    const menuId = getMenuLocaleId(name);
    return intl.formatMessage({ id: menuId, defaultMessage: name });
  };

  const renderMenuName = (item: API.Menu, level: number = 0) => {
    const indent = '　'.repeat(level);
    return `${indent}${localizeMenuName(item.name)}`;
  };

  const flattenMenu = (menus: API.Menu[], level: number = 0): any[] => {
    return menus.reduce((acc, menu) => {
      const flatMenu = { ...menu, displayName: renderMenuName(menu, level) };
      acc.push(flatMenu);
      const menuChildren = (menu as any).children;
      if (menuChildren && menuChildren.length > 0) {
        acc.push(...flattenMenu(menuChildren, level + 1));
      }
      return acc;
    }, [] as any[]);
  };

  const flatDataSource = flattenMenu(dataSource);

  return (
    <div className={styles.mobileContainer}>
      <div className={styles.toolbar}>
        <Access key="/menu/create" rootOnly>
          <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
            新建菜单
          </Button>
        </Access>
      </div>

      <List
        loading={loading}
        dataSource={flatDataSource}
        loadMore={loadMoreControl}
        renderItem={(item: any) => (
          <List.Item className={styles.listItem}>
            <Card className={styles.card} size="small">
              <div className={styles.cardHeader}>
                <span className={styles.name}>{item.displayName || item.name}</span>
                {getTypeTag(item.type || 'MENU')}
                {getStatusTag(item.status || 'enabled')}
              </div>
              
              <div className={styles.cardBody}>
                <div className={styles.field}>
                  <span className={styles.label}>路径:</span>
                  <span className={styles.value}>{item.path || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>图标:</span>
                  <span className={styles.value}>{item.icon || '-'}</span>
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>排序:</span>
                  <span className={styles.value}>{item.sort || '-'}</span>
                </div>
              </div>

              <div className={styles.cardActions}>
                <Space>
                  <Access key="/menu/edit" rootOnly>
                    <Button size="small" icon={<EditOutlined />} onClick={() => onEdit(item)}>
                      编辑
                    </Button>
                  </Access>
                  <Access key="/menu/delete" rootOnly>
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
        locale={{ emptyText: <Empty description="暂无菜单数据" /> }}
      />
    </div>
  );
};

export default MobileMenuList;
