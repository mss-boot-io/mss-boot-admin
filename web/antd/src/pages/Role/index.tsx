import { Access } from '@/components/MssBoot/Access';
import Auth from '@/components/MssBoot/Auth';
import { getMenuTree } from '@/services/admin/menu';
import {
  deleteRolesId,
  getRoles,
  getRolesId,
  postRoles,
  putRolesId,
} from '@/services/admin/role';
import {
  createRoleAuthorizationAdapter,
  isRoleAuthorizationRevisionConflictError,
  normalizeRoleAuthorizationPaths,
  type RoleAuthorizationResource,
} from '@/services/admin/roleAuthorization';
import { idRender } from '@/util/columnOptions';
import { fieldIntl } from '@/util/fieldIntl';
import { indexTitle } from '@/util/indexTitle';
import { useOption } from '@/hooks/useOption';
import { useResponsive } from '@/hooks/useResponsive';
import { requestPermissionRefresh } from '@/utils/permissionFreshness';
import { resolveCrudRouteID } from '@/utils/routeAccess';
import { PlusOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns, ProDescriptionsItemProps } from '@ant-design/pro-components';
import { DrawerForm, PageContainer, ProDescriptions, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, history, Link, useIntl, useParams } from '@umijs/max';
import { Alert, Button, Drawer, message, Modal, Popconfirm } from 'antd';
import { DataNode } from 'antd/es/tree';
import React, { useRef, useState } from 'react';
import MobileRoleList from './Mobile/RoleList';
import { getRoleActionDisabledState } from './roleActions';

const roleAuthorizationAdapter = createRoleAuthorizationAdapter();

const TableList: React.FC = () => {
  const [authModalOpen, setAuthModalOpen] = useState<boolean>(false);

  const [showDetail, setShowDetail] = useState<boolean>(false);

  const actionRef = useRef<ActionType>();
  const [currentRow, setCurrentRow] = useState<API.Role>();
  const [treeData, setTreeData] = useState<DataNode[]>([]);

  const [checkedKeys, setCheckedKeys] = useState<React.Key[]>([]);
  const [authorizationResource, setAuthorizationResource] =
    useState<RoleAuthorizationResource>();
  const [authorizationConflict, setAuthorizationConflict] = useState<{
    current?: RoleAuthorizationResource;
  }>();
  const [modalApi, modalContextHolder] = Modal.useModal();

  const { id: routeID } = useParams();
  const id = resolveCrudRouteID(routeID, history.location.pathname, '/role/create');

  const { isMobile } = useResponsive();
  const shouldLoadDesktopDependencies = !isMobile || !!id;
  const { valueEnum: statusValueEnum } = useOption('system', 'status', {
    enabled: shouldLoadDesktopDependencies,
  });

  const intl = useIntl();

  const columns: ProColumns<API.Role>[] = [
    {
      title: fieldIntl(intl, 'id'),
      dataIndex: 'id',
      hideInForm: true,
      render: (dom, entity) => {
        return idRender(dom, entity, setCurrentRow, setShowDetail);
      },
    },
    {
      title: fieldIntl(intl, 'name'),
      dataIndex: 'name',
    },
    {
      title: fieldIntl(intl, 'remark'),
      search: false,
      dataIndex: 'remark',
      valueType: 'textarea',
    },
    {
      title: fieldIntl(intl, 'root'),
      dataIndex: 'root',
      search: false,
      hideInForm: true,
      valueEnum: {
        false: {
          text: fieldIntl(intl, 'options.false'),
          status: 'false',
        },
        true: {
          text: fieldIntl(intl, 'options.true'),
          status: 'true',
        },
      },
    },
    {
      title: fieldIntl(intl, 'status'),
      dataIndex: 'status',
      valueEnum: statusValueEnum,
    },
    {
      title: fieldIntl(intl, 'updatedAt'),
      sorter: true,
      dataIndex: 'updatedAt',
      search: false,
      valueType: 'dateTime',
      hideInForm: true,
    },
    {
      title: <FormattedMessage id="pages.title.option" />,
      dataIndex: 'option',
      valueType: 'option',
      hideInDescriptions: true,
      hideInForm: true,
      render: (_, record) => [
        <Access key="/role/edit" rootOnly>
          {getRoleActionDisabledState(record).edit ? (
            <Button key="edit" disabled>
              <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
            </Button>
          ) : (
            <Link to={`/role/${record.id}`}>
              <Button key="edit">
                <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
              </Button>
            </Link>
          )}
        </Access>,
        <Access key="/role/auth" rootOnly>
          <Button
            key="auth"
            disabled={getRoleActionDisabledState(record).authorize}
            onClick={() => {
              setCurrentRow(record);
              setAuthModalOpen(true);
            }}
          >
            <FormattedMessage id="pages.role.auth.title" defaultMessage="Auth" />
          </Button>
        </Access>,
        <Access key="/role/delete" rootOnly>
          <Popconfirm
            key="delete"
            title={intl.formatMessage({
              id: 'pages.title.delete.confirm',
              defaultMessage: 'Confirm Delete',
            })}
            description={intl.formatMessage({
              id: 'pages.description.delete.confirm',
              defaultMessage: 'Are you sure to delete this record?',
            })}
            disabled={getRoleActionDisabledState(record).delete}
            onConfirm={async () => {
              await deleteRolesId({ id: record.id! });
              message
                .success(
                  intl.formatMessage({
                    id: 'pages.message.delete.success',
                    defaultMessage: 'Delete successfully!',
                  }),
                )
                .then(() => actionRef.current?.reload());
            }}
            okText={intl.formatMessage({ id: 'pages.title.ok', defaultMessage: 'OK' })}
            cancelText={intl.formatMessage({ id: 'pages.title.cancel', defaultMessage: 'Cancel' })}
          >
            <Button disabled={getRoleActionDisabledState(record).delete} key="delete.button">
              <FormattedMessage id="pages.title.delete" defaultMessage="Delete" />
            </Button>
          </Popconfirm>
        </Access>,
      ],
    },
  ];

  const transfer = (data: API.Menu[]): DataNode[] => {
    // @ts-ignore
    return data.map((item) => {
      const menuId = item.name?.startsWith('menu.') ? item.name : `menu.${item.name}`;
      return {
        title: intl.formatMessage({ id: menuId }),
        key: item.path === '/' ? item.name : item.path,
        // @ts-ignore
        children: item.children ? transfer(item.children) : null,
      };
    });
  };

  const onOpenChange = async (e: boolean) => {
    if (e) {
      const roleID = currentRow?.id;
      if (!roleID) {
        setAuthModalOpen(false);
        return;
      }
      try {
        const [data, resource] = await Promise.all([
          getMenuTree(),
          roleAuthorizationAdapter.load(roleID),
        ]);
        setTreeData(transfer(data));
        setAuthorizationResource(resource);
        setCheckedKeys(resource.paths);
        setAuthorizationConflict(undefined);
      } catch {
        message.error(intl.formatMessage({ id: 'pages.role.auth.failed' }));
        setAuthModalOpen(false);
      }
      return;
    }
    setTreeData([]);
    setCheckedKeys([]);
    setAuthorizationResource(undefined);
    setAuthorizationConflict(undefined);
    setAuthModalOpen(e);
  };

  const reloadLatestAuthorization = async () => {
    const roleID = authorizationResource?.roleID ?? currentRow?.id;
    if (!roleID) return;
    try {
      const latest =
        authorizationConflict?.current?.roleID === roleID
          ? authorizationConflict.current
          : await roleAuthorizationAdapter.load(roleID);
      setAuthorizationResource(latest);
      setCheckedKeys(latest.paths);
      setAuthorizationConflict(undefined);
      message.success(intl.formatMessage({ id: 'pages.role.auth.conflict.reloaded' }));
    } catch {
      message.error(intl.formatMessage({ id: 'pages.role.auth.failed' }));
    }
  };

  const onSubmit = async (params: any) => {
    if (!id) {
      return;
    }
    if (id === 'create') {
      await postRoles(params);
      message.success(
        intl.formatMessage({
          id: 'pages.message.create.success',
          defaultMessage: 'Create successfully!',
        }),
      );
      history.push('/role');
      return;
    }
    await putRolesId({ id }, params);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
    history.push('/role');
  };

  return (
    <PageContainer title={indexTitle(id)}>
      {modalContextHolder}
      {isMobile && !id ? (
        <MobileRoleList
          request={getRoles}
          onEdit={(record) => history.push(`/role/${record.id}`)}
          onCreate={() => history.push('/role/create')}
          onAuth={(record) => {
            setCurrentRow(record);
            setAuthModalOpen(true);
          }}
          onDelete={async (record) => {
            await deleteRolesId({ id: record.id! });
            message.success(intl.formatMessage({ id: 'pages.message.delete.success' }));
          }}
        />
      ) : (
        <ProTable<API.Role, API.getRolesParams>
          headerTitle={intl.formatMessage({
            id: 'pages.role.list.title',
            defaultMessage: 'Role List',
          })}
          actionRef={actionRef}
          rowKey="id"
          search={{
            labelWidth: 120,
          }}
          type={id ? 'form' : 'table'}
          onSubmit={id ? onSubmit : undefined}
          toolBarRender={() => [
            <Access key="/role/create" rootOnly>
              <Button type="primary" key="create">
                <Link type="primary" key="primary" to="/role/create">
                  <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
                </Link>
              </Button>
            </Access>,
          ]}
          form={
            id && id !== 'create'
              ? {
                  request: async () => {
                    const res = await getRolesId({ id });
                    return res;
                  },
                }
              : {
                  initialValues: {
                    status: 'enabled',
                  },
                }
          }
          request={getRoles}
          columns={columns}
        />
      )}
      <Drawer
        width={600}
        open={showDetail}
        onClose={() => {
          setCurrentRow(undefined);
          setShowDetail(false);
        }}
        closable={false}
      >
        {currentRow?.name && (
          <ProDescriptions<API.Role>
            column={2}
            title={currentRow?.name}
            request={async () => ({
              data: currentRow || {},
            })}
            params={{
              id: currentRow?.id,
            }}
            columns={columns as ProDescriptionsItemProps<API.Role>[]}
          />
        )}
      </Drawer>

      <DrawerForm
        onOpenChange={onOpenChange}
        title={intl.formatMessage({ id: 'pages.role.auth.title' })}
        open={authModalOpen}
        submitter={{
          submitButtonProps: {
            disabled: Boolean(authorizationConflict),
          },
        }}
        onFinish={async () => {
          if (!authorizationResource) return false;
          const paths = normalizeRoleAuthorizationPaths(
            checkedKeys.map((value) => value.toString()),
          );
          if (paths.length === 0) {
            const confirmed = await modalApi.confirm({
              title: intl.formatMessage({ id: 'pages.role.auth.clear.title' }),
              content: intl.formatMessage({ id: 'pages.role.auth.clear.description' }),
              okButtonProps: { danger: true },
              okText: intl.formatMessage({ id: 'pages.title.ok' }),
              cancelText: intl.formatMessage({ id: 'pages.title.cancel' }),
            });
            if (!confirmed) return false;
          }

          try {
            const next = await roleAuthorizationAdapter.save(paths, authorizationResource);
            setAuthorizationResource(next);
            setCheckedKeys(next.paths);
            setAuthorizationConflict(undefined);
            requestPermissionRefresh();
            message.success(intl.formatMessage({ id: 'pages.role.auth.success' }));
            return true;
          } catch (error) {
            if (isRoleAuthorizationRevisionConflictError(error)) {
              let latest = error.current;
              if (!latest) {
                try {
                  latest = await roleAuthorizationAdapter.load(authorizationResource.roleID);
                } catch {
                  // The conflict state remains visible and the explicit reload
                  // action retries if the latest resource cannot be fetched yet.
                }
              }
              setAuthorizationConflict({ current: latest });
              message.warning(intl.formatMessage({ id: 'pages.role.auth.conflict.title' }));
              return false;
            }
            message.error(intl.formatMessage({ id: 'pages.role.auth.failed' }));
            return false;
          }
        }}
      >
        {authorizationConflict && (
          <Alert
            showIcon
            type="warning"
            message={intl.formatMessage({ id: 'pages.role.auth.conflict.title' })}
            description={intl.formatMessage({ id: 'pages.role.auth.conflict.description' })}
            action={
              <Button size="small" onClick={() => void reloadLatestAuthorization()}>
                {intl.formatMessage({ id: 'pages.role.auth.conflict.reload' })}
              </Button>
            }
            style={{ marginBottom: 16 }}
          />
        )}
        <Auth values={treeData} setCheckedKeys={setCheckedKeys} checkedKeys={checkedKeys} />
      </DrawerForm>
    </PageContainer>
  );
};

export default TableList;
