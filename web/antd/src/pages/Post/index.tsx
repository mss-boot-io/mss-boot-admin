import { Access } from '@/components/MssBoot/Access';
import { idRender } from '@/util/columnOptions';
import { indexTitle } from '@/util/indexTitle';
import { useOption } from '@/hooks/useOption';
import { useResponsive } from '@/hooks/useResponsive';
import { PlusOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns, ProDescriptionsItemProps } from '@ant-design/pro-components';
import { PageContainer, ProDescriptions, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, history, Link, useIntl, useParams } from '@umijs/max';
import { Button, Drawer, message, Popconfirm, TreeSelect } from 'antd';
import { DataNode } from 'antd/es/tree';
import React, { useEffect, useRef, useState } from 'react';
import { fieldIntl } from '@/util/fieldIntl';
import { deletePostsId, getPosts, getPostsId, postPosts, putPostsId } from '@/services/admin/post';
import MobilePostList from './Mobile/PostList';

const TableList: React.FC = () => {
  const [showDetail, setShowDetail] = useState<boolean>(false);

  const actionRef = useRef<ActionType>();
  const [currentRow, setCurrentRow] = useState<API.Role>();
  const [list, setList] = useState<[]>([]);
  const { id } = useParams();

  const { valueEnum: statusValueEnum } = useOption('system', 'status');
  const { valueEnum: dataScopeValueEnum } = useOption('permission', 'dataScope');
  const { isMobile } = useResponsive();

  const intl = useIntl();

  const columns: ProColumns<API.Post>[] = [
    {
      title: fieldIntl(intl, 'id'),
      dataIndex: 'id',
      hideInForm: true,
      search: false,
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
            placeholder={intl.formatMessage({
              id: 'pages.post.parent.placeholder',
              defaultMessage: 'Please select parent post',
            })}
            allowClear
            treeDefaultExpandAll
            treeData={list}
          />
        );
      },
    },
    {
      title: fieldIntl(intl, 'name'),
      dataIndex: 'name',
      formItemProps: {
        rules: [
          {
            required: true,
            message: intl.formatMessage({
              id: 'pages.post.name.required',
              defaultMessage: 'Post name is required',
            }),
          },
        ],
      },
    },
    {
      title: fieldIntl(intl, 'code'),
      dataIndex: 'code',
      formItemProps: {
        rules: [
          {
            required: true,
            message: intl.formatMessage({
              id: 'pages.post.code.required',
              defaultMessage: 'Post code is required',
            }),
          },
        ],
      },
    },
    {
      title: fieldIntl(intl, 'dataScope'),
      dataIndex: 'dataScope',
      search: false,
      valueEnum: dataScopeValueEnum,
      formItemProps: {
        rules: [
          {
            required: true,
            message: intl.formatMessage({
              id: 'pages.post.dataScope.required',
              defaultMessage: 'Data scope is required',
            }),
          },
        ],
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
      valueEnum: statusValueEnum,
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
        <Access key="/posts/edit">
          <Link to={`/posts/${record.id}`} key="edit">
            <Button key="edit">
              <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
            </Button>
          </Link>
        </Access>,
        <Access key="/posts/delete">
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
              await deletePostsId({ id: record.id! });
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

  const transferTree = (data: API.Post[], self: string): DataNode[] => {
    // @ts-ignore
    return data.map((item) => {
      return {
        title: item.name,
        value: item.id,
        disabled: item.id === self,
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
      await postPosts(params);
      message.success(
        intl.formatMessage({
          id: 'pages.message.create.success',
          defaultMessage: 'Create successfully!',
        }),
      );
      history.push('/posts');
      return;
    }
    await putPostsId({ id }, params);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
    history.push('/posts');
  };

  useEffect(() => {
    if (id) {
      getPosts({ pageSize: 1000, parentID: '' }).then((res) => {
        // @ts-ignore
        setList(transferTree(res.data!, id));
      });
    }
  }, [id]);

  return (
    <PageContainer title={indexTitle(id)}>
      {isMobile && !id ? (
        <MobilePostList
          request={getPosts}
          onEdit={(record) => history.push(`/posts/${record.id}`)}
          onCreate={() => history.push('/posts/create')}
          onDelete={async (record) => {
            await deletePostsId({ id: record.id! });
            message.success(intl.formatMessage({ id: 'pages.message.delete.success' }));
          }}
          dataScopeValueEnum={dataScopeValueEnum}
        />
      ) : (
        <ProTable<API.Post, API.getPostsParams>
        headerTitle={intl.formatMessage({
          id: 'pages.post.list.title',
          defaultMessage: 'Post List',
        })}
        actionRef={actionRef}
        rowKey="id"
        search={{
          labelWidth: 120,
        }}
        type={id ? 'form' : 'table'}
        onSubmit={id ? onSubmit : undefined}
        toolBarRender={() => [
          <Access key="/posts/create">
            <Button type="primary" key="create">
              <Link type="primary" to="/posts/create" key="/posts/create">
                <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
              </Link>
            </Button>
          </Access>,
        ]}
        form={
          id && id !== 'create'
            ? {
                request: async () => {
                  const res = await getPostsId({ id });
                  return res;
                },
              }
            : {
                initialValues: {
                  status: 'enabled',
                },
              }
        }
        request={getPosts}
        params={{ parentID: '' }}
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
          <ProDescriptions<API.Post>
            column={2}
            title={currentRow?.name}
            request={async (params) => {
              // @ts-ignore
              const res = await getPostsId(params);
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
    </PageContainer>
  );
};

export default TableList;
