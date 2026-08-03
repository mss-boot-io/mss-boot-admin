import { Access } from '@/components/MssBoot/Access';
import Auth from '@/components/MssBoot/Auth';
import { getApis } from '@/services/admin/api';
import {
  deleteMenusId,
  getMenuApiId,
  getMenus,
  getMenusId,
  postMenuBindApi,
  postMenus,
  putMenusId,
} from '@/services/admin/menu';
import { idRender } from '@/util/columnOptions';
import { indexTitle } from '@/util/indexTitle';
import { useResponsive } from '@/hooks/useResponsive';
import { PlusOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns, ProDescriptionsItemProps } from '@ant-design/pro-components';
import { DrawerForm, PageContainer, ProDescriptions, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, history, Link, useIntl, useParams } from '@umijs/max';
import { Button, Drawer, message, Popconfirm, TreeSelect } from 'antd';
import { DataNode } from 'antd/es/tree';
import React, { useEffect, useRef, useState } from 'react';
import { fieldIntl } from '@/util/fieldIntl';
import MobileMenuList from './Mobile/MenuList';

const TableList: React.FC = () => {
  const [showDetail, setShowDetail] = useState<boolean>(false);

  const actionRef = useRef<ActionType>();
  const [currentRow, setCurrentRow] = useState<API.Role>();

  const [authModalOpen, setAuthModalOpen] = useState<boolean>(false);
  const { isMobile } = useResponsive();
  const [treeData, setTreeData] = useState<DataNode[]>([]);

  const [checkedKeys, setCheckedKeys] = useState<React.Key[]>([]);

  const [list, setList] = useState<[]>([]);

  const { id } = useParams();

  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const columns: ProColumns<API.Menu>[] = [
    {
      title: fieldIntl(intl, 'id'),
      dataIndex: 'id',
      hideInForm: true,
      render: (dom, entity) => {
        return idRender(dom, entity, setCurrentRow, setShowDetail);
      },
    },
    {
      title: fieldIntl(intl, 'parentID'),
      search: false,
      hideInTable: true,
      dataIndex: 'parentID',
      renderFormItem: () => {
        return (
          <TreeSelect
            showSearch
            style={{ width: '100%' }}
            dropdownStyle={{ maxHeight: 400, overflow: 'auto' }}
            placeholder={fieldIntl(intl, 'parent.placeholder')}
            allowClear
            treeDefaultExpandAll
            // onChange={onChange}
            treeData={list}
          />
        );
      },
    },
    {
      title: fieldIntl(intl, 'type'),
      dataIndex: 'type',
      search: false,
      valueEnum: {
        DIRECTORY: {
          text: fieldIntl(intl, 'options.directory'),
          color: 'blue',
          status: 'DIRECTORY',
        },
        MENU: {
          text: fieldIntl(intl, 'options.menu'),
          color: 'green',
          status: 'MENU',
        },
        COMPONENT: {
          text: fieldIntl(intl, 'options.component'),
          color: 'yellow',
          status: 'COMPONENT',
        },
        API: {
          text: fieldIntl(intl, 'options.api'),
          color: 'red',
          status: 'API',
        },
      },
    },
    // {
    //   title: fieldIntl(intl, 'icon'),
    //   dataIndex: 'icon',
    //   search: false,
    //   render: (dom) => {
    //     return <span className={`iconfont ${dom}`} />;
    //   },
    // },
    {
      title: fieldIntl(intl, 'name'),
      dataIndex: 'name',
      formItemProps: {
        rules: [{ required: true }],
      },
      render: (dom) => {
        const menuName = String(dom ?? '');
        const menuId = menuName.startsWith('menu.') ? menuName : `menu.${menuName}`;
        return intl.formatMessage({ id: menuId });
      },
    },

    {
      title: fieldIntl(intl, 'path'),
      search: false,
      dataIndex: 'path',
      // valueType: 'textarea',
    },
    {
      title: fieldIntl(intl, 'hideInMenu'),
      dataIndex: 'hideInMenu',
      search: false,
      valueType: 'switch',
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
      title: fieldIntl(intl, 'sort'),
      dataIndex: 'sort',
      search: false,
      valueType: 'digit',
    },
    {
      title: fieldIntl(intl, 'status'),
      dataIndex: 'status',
      valueEnum: {
        enabled: {
          text: fieldIntl(intl, 'options.enabled'),
          status: 'enabled',
        },
        disabled: {
          text: fieldIntl(intl, 'options.disabled'),
          status: 'disabled',
        },
        locked: {
          text: fieldIntl(intl, 'options.locked'),
          status: 'locked',
        },
      },
    },
    {
      title: fieldIntl(intl, 'updatedAt'),
      sorter: true,
      dataIndex: 'updatedAt',
      search: false,
      hideInForm: true,
      valueType: 'dateTime',
    },
    {
      title: <FormattedMessage id="pages.title.option" />,
      dataIndex: 'option',
      valueType: 'option',
      hideInDescriptions: true,
      hideInForm: true,
      render: (_, record) => [
        <Access key="/menu/edit">
          <Link to={`/menu/${record.id}`} key="edit">
            <Button key="edit">
              <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
            </Button>
          </Link>
        </Access>,
        <Access key="/menu/bind-api">
          <Button
            disabled={record.type === 'DIRECTORY'}
            key="bind-api"
            onClick={() => {
              setCurrentRow(record);
              setAuthModalOpen(true);
            }}
          >
            <FormattedMessage id="pages.user.binding.api" defaultMessage="Bingding API" />
          </Button>
        </Access>,
        <Access key="/menu/delete">
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
            onConfirm={async () => {
              await deleteMenusId({ id: record.id! });
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
            <Button key="delete.button">
              <FormattedMessage id="pages.title.delete" defaultMessage="Delete" />
            </Button>
          </Popconfirm>
        </Access>,
      ],
    },
  ];

  const transfer = (data: API.API[]): DataNode[] => {
    // @ts-ignore
    return data.map((item) => {
      return {
        title: `${item.method}---${item.path}`,
        key: `${item.method}---${item.path}`,
        // @ts-ignore
        children: item.children ? transfer(item.children) : null,
      };
    });
  };

  const onOpenChange = async (e: boolean) => {
    if (e) {
      const { data } = await getApis({ pageSize: 9999 });
      const res = transfer(data!);
      setTreeData(res);
      //get checkedKeys
      const checkedRes = await getMenuApiId({
        id: currentRow?.id ?? '',
      });
      if (checkedRes) {
        const checkedKeys: React.Key[] = [];
        checkedRes?.forEach((value) => {
          checkedKeys.push(`${value.method}---${value.path}`);
        });
        setCheckedKeys(checkedKeys);
      }
      return;
    }
    setTreeData([]);
    setAuthModalOpen(e);
  };

  const transferTree = (data: API.Menu[]): DataNode[] => {
    // @ts-ignore
    return data.map((item) => {
      const menuId = item.name?.startsWith('menu.') ? item.name : `menu.${item.name}`;
      return {
        title: intl.formatMessage({ id: menuId }),
        value: item.id,
        // @ts-ignore
        children: item.children ? transferTree(item.children) : null,
      };
    });
  };

  const onSubmit = async (params: any) => {
    if (!id) {
      return;
    }
    if (id === 'create') {
      await postMenus(params);
      message.success(
        intl.formatMessage({
          id: 'pages.message.create.success',
          defaultMessage: 'Create successfully!',
        }),
      );
      history.push('/menu');
      return;
    }
    await putMenusId({ id }, params);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
    history.push('/menu');
  };

  useEffect(() => {
    if (id) {
      getMenus({ pageSize: 1000, type: ['DIRECTORY'], show: true }).then((res) => {
        // @ts-ignore
        setList(transferTree(res.data!));
      });
    }
  }, [id]);

  return (
    <PageContainer title={indexTitle(id)}>
      {isMobile && !id ? (
        <MobileMenuList
          request={getMenus}
          onEdit={(record) => history.push(`/menu/${record.id}`)}
          onCreate={() => history.push('/menu/create')}
          onDelete={async (record) => {
            await deleteMenusId({ id: record.id! });
            message.success(intl.formatMessage({ id: 'pages.message.delete.success' }));
          }}
        />
      ) : (
        <ProTable<API.Menu, API.getMenusParams>
        headerTitle={intl.formatMessage({
          id: 'pages.menu.list.title',
          defaultMessage: 'Menu List',
        })}
        actionRef={actionRef}
        rowKey="id"
        search={{
          labelWidth: 120,
        }}
        type={id ? 'form' : 'table'}
        onSubmit={id ? onSubmit : undefined}
        // onSubmit={async (params) => {
        //   console.log(params);
        // }}
        toolBarRender={() => [
          <Access key="/menu/create">
            <Link to="/menu/create" key="create">
              <Button type="primary" key="create">
                <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
              </Button>
            </Link>
          </Access>,
        ]}
        form={
          id && id !== 'create'
            ? {
                request: async () => {
                  return await getMenusId({ id });
                },
              }
            : undefined
        }
        request={getMenus}
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
          <ProDescriptions<API.Menu>
            column={2}
            title={currentRow?.name}
            request={async (params) => {
              // @ts-ignore
              const res = await getMenusId(params);
              res.name = currentRow?.name;
              return {
                data: res,
              };
            }}
            params={{
              id: currentRow?.id,
            }}
            columns={columns as ProDescriptionsItemProps<API.Menu>[]}
          />
        )}
      </Drawer>
      <DrawerForm
        onOpenChange={onOpenChange}
        title="绑定API"
        open={authModalOpen}
        onFinish={async () => {
          const paths: string[] = [];
          checkedKeys.forEach((value) => {
            paths.push(value.toString());
          });

          await postMenuBindApi({
            menuID: currentRow?.id ?? '',
            paths,
          });
          message.success(intl.formatMessage({ id: 'pages.menu.binding.api.success' }));
          return true;
        }}
      >
        <Auth values={treeData} setCheckedKeys={setCheckedKeys} checkedKeys={checkedKeys} />
      </DrawerForm>
    </PageContainer>
  );
};

export default TableList;
