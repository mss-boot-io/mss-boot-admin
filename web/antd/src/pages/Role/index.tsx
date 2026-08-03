import { Access } from '@/components/MssBoot/Access';
import Auth from '@/components/MssBoot/Auth';
import { getMenuTree } from '@/services/admin/menu';
import {
  deleteRolesId,
  getRoleAuthorizeRoleId,
  getRoles,
  getRolesId,
  postRoleAuthorizeRoleId,
  postRoles,
  putRolesId,
} from '@/services/admin/role';
import { idRender } from '@/util/columnOptions';
import { fieldIntl } from '@/util/fieldIntl';
import { indexTitle } from '@/util/indexTitle';
import { useOption } from '@/hooks/useOption';
import { useResponsive } from '@/hooks/useResponsive';
import { PlusOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns, ProDescriptionsItemProps } from '@ant-design/pro-components';
import { DrawerForm, PageContainer, ProDescriptions, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, history, Link, useIntl, useParams } from '@umijs/max';
import { Button, Drawer, message, Popconfirm } from 'antd';
import { DataNode } from 'antd/es/tree';
import React, { useRef, useState } from 'react';
import MobileRoleList from './Mobile/RoleList';

const TableList: React.FC = () => {
  const [authModalOpen, setAuthModalOpen] = useState<boolean>(false);

  const [showDetail, setShowDetail] = useState<boolean>(false);

  const actionRef = useRef<ActionType>();
  const [currentRow, setCurrentRow] = useState<API.Role>();
  const [treeData, setTreeData] = useState<DataNode[]>([]);

  const [checkedKeys, setCheckedKeys] = useState<React.Key[]>([]);

  const { id } = useParams();

  const { valueEnum: statusValueEnum } = useOption('system', 'status');
  const { isMobile } = useResponsive();

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
        <Access key="/role/edit">
          <Link to={`/role/${record.id}`}>
            <Button key="edit">
              <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
            </Button>
          </Link>
        </Access>,
        <Access key="/role/auth">
          <Button
            key="auth"
            disabled={record.root}
            onClick={() => {
              setAuthModalOpen(true);
              setCurrentRow(record);
            }}
          >
            <FormattedMessage id="pages.role.auth.title" defaultMessage="Auth" />
          </Button>
        </Access>,
        <Access key="/role/delete">
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
            disabled={record.root}
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
            <Button disabled={record.root} key="delete.button">
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
      const data = await getMenuTree();
      setTreeData(transfer(data));
      //get checkedKeys
      const checkedRes = await getRoleAuthorizeRoleId({
        roleID: currentRow?.id ?? '',
      });
      if (checkedRes) {
        const checkedKeys: React.Key[] = [];
        checkedRes.paths?.forEach((value) => {
          checkedKeys.push(value);
        });
        setCheckedKeys(checkedKeys);
      }
      return;
    }
    setTreeData([]);
    setAuthModalOpen(e);
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
            <Access key="/role/create">
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
        onFinish={async () => {
          const paths: string[] = [];
          checkedKeys.forEach((value) => {
            paths.push(value.toString());
          });

          await postRoleAuthorizeRoleId(
            {
              roleID: currentRow?.id ?? '',
            },
            {
              paths,
            },
          );
          message.success(intl.formatMessage({ id: 'pages.role.auth.success' }));
        }}
      >
        <Auth values={treeData} setCheckedKeys={setCheckedKeys} checkedKeys={checkedKeys} />
      </DrawerForm>
    </PageContainer>
  );
};

export default TableList;
