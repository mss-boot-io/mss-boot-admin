import { Access } from '@/components/MssBoot/Access';
import { getRoles } from '@/services/admin/role';
import { deleteUsersId, getUsers, getUsersId, postUsers, putUsersId } from '@/services/admin/user';
import { idRender } from '@/util/columnOptions';
import { indexTitle } from '@/util/indexTitle';
import { toOptions } from '@/util/toOptions';
import { useOption } from '@/hooks/useOption';
import { useResponsive } from '@/hooks/useResponsive';
import { resolveCrudRouteID } from '@/utils/routeAccess';
import { PlusOutlined } from '@ant-design/icons';
import MobileUserList from './Mobile/UserList';
import type { ActionType, ProColumns, ProDescriptionsItemProps } from '@ant-design/pro-components';
import { PageContainer, ProDescriptions, ProTable } from '@ant-design/pro-components';
import {
  FormattedMessage,
  history,
  Link,
  useIntl,
  useModel,
  useParams,
  useRequest,
} from '@umijs/max';
import { Button, Card, Drawer, message, Popconfirm, Result, Skeleton, TreeSelect } from 'antd';
import React, { useRef, useState } from 'react';
import { fieldIntl } from '@/util/fieldIntl';
import { getDepartments } from '@/services/admin/department';
import { getPosts } from '@/services/admin/post';
import {
  buildTreeOptions,
  findTreeOptionLabel,
  type TreeOptionNode,
  type TreeOptionRecord,
} from './treeOptions';
import { loadUserDependencies, resolveUserDependencyAccess } from './dependencies';

type UserFormDependencies = {
  roleOptions: API.Role[];
  postOptions: TreeOptionNode[];
  deptOptions: TreeOptionNode[];
};

const UserList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [showDetail, setShowDetail] = useState<boolean>(false);
  const [currentRow, setCurrentRow] = useState<API.Role>();
  const { id: routeID } = useParams();
  const id = resolveCrudRouteID(routeID, history.location.pathname, '/users/control/create');
  const [tableLoading, setTableLoading] = useState(true);
  const { isMobile } = useResponsive();
  const { initialState } = useModel('@@initialState');
  const isForm = Boolean(id);
  const dependencyAccess = resolveUserDependencyAccess(initialState?.currentUser, {
    isForm,
    isMobile,
  });
  const shouldLoadDependencies = Object.values(dependencyAccess).some(Boolean);
  const shouldLoadDesktopDependencies = !isMobile || !!id;
  const {
    valueEnum: statusValueEnum,
    loading: statusOptionLoading,
    error: statusOptionError,
    refresh: refreshStatusOption,
  } = useOption('system', 'status', {
    enabled: shouldLoadDesktopDependencies,
  });

  const intl = useIntl();

  const {
    data: dependencies,
    error: dependencyError,
    loading,
    refresh: refreshDependencies,
  } = useRequest(
    async () => {
      const { values } = await loadUserDependencies(
        dependencyAccess,
        {
          roles: () => getRoles({ pageSize: 1000 }),
          posts: () => getPosts({ pageSize: 1000, parentID: '' }),
          departments: () => getDepartments({ pageSize: 1000, parentID: '' }),
        },
        isForm,
      );
      const roles = values.roles as Awaited<ReturnType<typeof getRoles>> | undefined;
      const posts = values.posts as Awaited<ReturnType<typeof getPosts>> | undefined;
      const departments = values.departments as
        | Awaited<ReturnType<typeof getDepartments>>
        | undefined;
      return {
        roleOptions: roles?.data || [],
        postOptions: buildTreeOptions((posts?.data || []) as TreeOptionRecord[], id),
        deptOptions: buildTreeOptions((departments?.data || []) as TreeOptionRecord[], id),
      } satisfies UserFormDependencies;
    },
    {
      ready: shouldLoadDependencies,
      refreshDeps: [
        id,
        isMobile,
        dependencyAccess.roles,
        dependencyAccess.posts,
        dependencyAccess.departments,
      ],
    },
  );
  const typedDependencies = dependencies as UserFormDependencies | undefined;
  const roleOptions = typedDependencies?.roleOptions || [];
  const postOptions = typedDependencies?.postOptions || [];
  const deptOptions = typedDependencies?.deptOptions || [];
  const dependenciesLoading = isForm && (loading || statusOptionLoading);
  const dependenciesError = isForm && (dependencyError || statusOptionError);
  const retryDependencies = () => {
    refreshDependencies();
    refreshStatusOption();
  };

  if (isMobile && !id) {
    return (
      <PageContainer
        title={intl.formatMessage({
          id: 'pages.user.list.title',
          defaultMessage: 'User List',
        })}
      >
        <MobileUserList />
      </PageContainer>
    );
  }

  const columns: ProColumns<API.User>[] = [
    {
      title: fieldIntl(intl, 'id'),
      dataIndex: 'id',
      width: 220,
      hideInForm: true,
      render: (dom, entity) => {
        return idRender(dom, entity, setCurrentRow, setShowDetail);
      },
    },
    {
      title: fieldIntl(intl, 'roleID'),
      dataIndex: 'roleID',
      width: 120,
      search: false,
      valueType: 'select',
      valueEnum: toOptions(roleOptions),
      renderText: (val) =>
        roleOptions.find((option) => option.id === val)?.name || String(val ?? ''),
    },
    {
      title: fieldIntl(intl, 'department'),
      dataIndex: 'departmentID',
      width: 150,
      valueType: 'select',
      renderText: (val) => findTreeOptionLabel(deptOptions, val) || String(val ?? ''),
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
            treeData={deptOptions}
          />
        );
      },
    },
    {
      title: fieldIntl(intl, 'post'),
      dataIndex: 'postID',
      width: 150,
      renderText: (val) => findTreeOptionLabel(postOptions, val) || String(val ?? ''),
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
            treeData={postOptions}
          />
        );
      },
    },
    {
      title: fieldIntl(intl, 'avatar'),
      dataIndex: 'avatar',
      width: 72,
      search: false,
      valueType: 'avatar',
      hideInForm: true,
    },
    {
      title: fieldIntl(intl, 'username'),
      dataIndex: 'username',
      width: 140,
      formItemProps: {
        rules: [
          { required: true },
          { min: 3 },
          { max: 20 },
          {
            pattern: /^[a-zA-Z0-9_]+$/,
            message: intl.formatMessage({
              id: 'pages.message.username.rule.pattern',
              defaultMessage: '用户名只能包含字母、数字和下划线',
            }),
          },
        ],
      },
    },
    {
      title: fieldIntl(intl, 'name'),
      dataIndex: 'name',
      width: 140,
    },
    {
      title: fieldIntl(intl, 'email'),
      dataIndex: 'email',
      width: 220,
      formItemProps: {
        rules: [{ required: true }, { type: 'email' }],
      },
    },
    {
      title: fieldIntl(intl, 'password'),
      dataIndex: 'password',
      search: false,
      hideInTable: true,
      hideInDescriptions: true,
      valueType: 'password',
      formItemProps: {
        rules: [
          { min: 8 },
          { max: 20 },
          {
            pattern: /[a-zA-Z]/,
            message: intl.formatMessage({
              id: 'pages.message.password.rule.pattern.letters',
              defaultMessage: 'The password must contain letters',
            }),
          },
          {
            pattern: /[0-9]/,
            message: intl.formatMessage({
              id: 'pages.message.password.rule.pattern.numbers',
              defaultMessage: 'The password must contain numbers',
            }),
          },
        ],
      },
    },
    {
      title: fieldIntl(intl, 'confirmPassword'),
      dataIndex: 'confirmPassword',
      search: false,
      hideInTable: true,
      valueType: 'password',
      hideInDescriptions: true,
      formItemProps: {
        rules: [
          ({ getFieldValue }) => ({
            validator(_, value) {
              if (!value || getFieldValue('password') === value) {
                return Promise.resolve();
              }
              return Promise.reject(
                new Error(
                  intl.formatMessage({
                    id: 'pages.message.password.confirm.failed',
                    defaultMessage: 'The two passwords that you entered do not match!',
                  }),
                ),
              );
            },
          }),
        ],
      },
    },
    {
      title: fieldIntl(intl, 'status'),
      dataIndex: 'status',
      width: 100,
      valueEnum: statusValueEnum,
    },
    // {
    //   title: '用户来源',
    //   dataIndex: 'type',
    //   search: false,
    //   render: (dom) => {
    //     console.log(dom);
    //     return (
    //       <>
    //         {dom === 'admin' && <FormOutlined />}
    //         {dom === 'github' && <GithubOutlined />}
    //       </>
    //     );
    //   },
    // },
    {
      title: fieldIntl(intl, 'updatedAt'),
      sorter: true,
      dataIndex: 'updatedAt',
      width: 180,
      search: false,
      valueType: 'dateTime',
      hideInForm: true,
    },
    {
      title: <FormattedMessage id="pages.title.option" />,
      dataIndex: 'option',
      valueType: 'option',
      width: 236,
      fixed: 'right',
      hideInDescriptions: true,
      hideInForm: true,
      render: (_, record) => [
        <Access key="/users/edit" rootOnly>
          <Link to={`/users/control/${record.id}`}>
            <Button key="edit">
              <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
            </Button>
          </Link>
        </Access>,
        <Access
          key="/users/password-reset"
          permission="/users/password-reset"
          rootOnly={record.role?.root === true}
        >
          <Link to={`/users/password-reset/${record.id}/`}>
            <Button key="passwordReset">
              <FormattedMessage id="pages.title.password.reset" defaultMessage="ResetPassword" />
            </Button>
          </Link>
        </Access>,
        <Access key="/users/delete" rootOnly>
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
              await deleteUsersId({ id: record.id! });
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

  const onSubmit = async (params: any) => {
    if (!id) {
      return;
    }
    if (id === 'create') {
      await postUsers(params);
      message.success(
        intl.formatMessage({
          id: 'pages.message.create.success',
          defaultMessage: 'Create successfully!',
        }),
      );
      history.push('/users');
      return;
    }
    await putUsersId({ id }, params);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
    history.push('/users');
  };

  return dependenciesLoading ? (
    <PageContainer title={indexTitle(id)}>
      <Card>
        <Skeleton active paragraph={{ rows: 8 }} />
      </Card>
    </PageContainer>
  ) : dependenciesError ? (
    <PageContainer title={indexTitle(id)}>
      <Card>
        <Result
          status="error"
          title={intl.formatMessage({
            id: 'pages.user.dependencies.error.title',
            defaultMessage: 'Unable to load user form options',
          })}
          subTitle={intl.formatMessage({
            id: 'pages.user.dependencies.error.description',
            defaultMessage: 'Roles, departments, or posts could not be loaded. Please try again.',
          })}
          extra={
            <Button type="primary" onClick={retryDependencies}>
              {intl.formatMessage({
                id: 'pages.user.dependencies.error.retry',
                defaultMessage: 'Retry',
              })}
            </Button>
          }
        />
      </Card>
    </PageContainer>
  ) : (
    <PageContainer
      title={
        id
          ? indexTitle(id)
          : intl.formatMessage({
              id: 'pages.user.list.title',
              defaultMessage: 'User List',
            })
      }
    >
      <ProTable<API.User, API.Page>
        headerTitle={intl.formatMessage({
          id: 'pages.user.list.title',
          defaultMessage: 'User List',
        })}
        actionRef={actionRef}
        loading={tableLoading}
        onLoadingChange={(nextLoading) => setTableLoading(Boolean(nextLoading))}
        rowKey="id"
        size="small"
        scroll={{ x: 'max-content' }}
        search={{
          labelWidth: 120,
        }}
        type={id ? 'form' : 'table'}
        onSubmit={id ? onSubmit : undefined}
        toolBarRender={() => [
          <Access key="/users/create" rootOnly>
            <Button type="primary" key="create">
              <Link type="primary" key="primary" to="/users/control/create">
                <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
              </Link>
            </Button>
          </Access>,
        ]}
        form={
          id && id !== 'create'
            ? {
                request: async () => {
                  const res = await getUsersId({ id });
                  return res;
                },
              }
            : undefined
        }
        request={getUsers}
        columns={columns}
      ></ProTable>

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
          <ProDescriptions<API.User>
            column={1}
            title={currentRow?.name}
            request={async (params) => {
              // @ts-ignore
              const res = await getUsersId(params);
              res.name = currentRow?.name;
              return {
                data: res,
              };
            }}
            params={{
              id: currentRow?.id,
            }}
            columns={columns as ProDescriptionsItemProps<API.User>[]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default UserList;
