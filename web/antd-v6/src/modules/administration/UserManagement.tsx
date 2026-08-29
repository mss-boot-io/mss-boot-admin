import { getRequestErrorMessage } from '@mss-admin-core/shared/api/errors';
import {
  finishManagementRouteIntent,
  type ManagementRouteIntent,
  useManagementRouteIntent,
} from '@mss-admin-core/shared/navigation/managementRoute';
import type { PagePresentationRuntime } from '@mss-admin-core/shared/presentation/runtime';
import { queryKeys } from '@mss-admin-core/shared/query/client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import type { TableColumnsType } from 'antd';
import { Alert, App, Avatar, Button, Form, Input, Modal, Popconfirm, Select, Space } from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import AdministrationTable, { AdministrationStatusTag } from './AdministrationTable';
import { administrationAPI } from './api';
import {
  type AdministrationListParams,
  administrationReferenceName,
  administrationSelectOptions,
  flattenAdministrationTree,
  isAdminPageSize,
  type UserSummary,
  type UserWriteValues,
} from './contract';
import { useAdministrationCatalog, useAdministrationPage } from './query';
import {
  userPresentationListComponents,
  userPresentationMobileFields,
  userPresentationSearchComponents,
} from './userPresentation';

interface UserManagementProps {
  canCreate: boolean;
  canDelete: boolean;
  canEdit: boolean;
  canResetPassword: boolean;
  presentationRuntime: PagePresentationRuntime;
  routeIntent?: ManagementRouteIntent;
}

const initialParams: AdministrationListParams = {
  current: 1,
  pageSize: 20,
  status: 'all',
};

export default function UserManagement({
  canCreate,
  canDelete,
  canEdit,
  canResetPassword,
  presentationRuntime,
  routeIntent,
}: UserManagementProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const presentation = presentationRuntime.model;
  const configuredPageSize = isAdminPageSize(presentation.list.pageSize)
    ? presentation.list.pageSize
    : initialParams.pageSize;
  const [params, setParams] = useState<AdministrationListParams>(() => ({
    ...initialParams,
    pageSize: configuredPageSize,
  }));
  const queryWasChanged = useRef(false);
  const appliedPresentationPageSize = useRef(configuredPageSize);
  const updateParams: typeof setParams = (value) => {
    queryWasChanged.current = true;
    setParams(value);
  };
  useEffect(() => {
    if (queryWasChanged.current || appliedPresentationPageSize.current === configuredPageSize) {
      return;
    }
    setParams((current) => ({ ...current, current: 1, pageSize: configuredPageSize }));
    appliedPresentationPageSize.current = configuredPageSize;
  }, [configuredPageSize]);
  const users = useAdministrationPage('users', params);
  const roles = useAdministrationCatalog('roles');
  const departments = useAdministrationCatalog('departments');
  const posts = useAdministrationCatalog('posts');
  const rolesByID = useMemo(
    () => new Map((roles.data ?? []).map((role) => [role.id, role])),
    [roles.data],
  );
  const roleNamesByID = useMemo(
    () => new Map((roles.data ?? []).map((role) => [role.id, role.name] as const)),
    [roles.data],
  );
  const departmentNamesByID = useMemo(
    () =>
      new Map(
        flattenAdministrationTree(departments.data ?? []).map((department) => [
          department.id,
          department.name,
        ]),
      ),
    [departments.data],
  );
  const postNamesByID = useMemo(
    () => new Map(flattenAdministrationTree(posts.data ?? []).map((post) => [post.id, post.name])),
    [posts.data],
  );
  const departmentOptions = useMemo(
    () => administrationSelectOptions(departments.data ?? []),
    [departments.data],
  );
  const postOptions = useMemo(() => administrationSelectOptions(posts.data ?? []), [posts.data]);
  const [editing, setEditing] = useState<UserSummary | 'create'>();
  const [form] = Form.useForm<UserWriteValues & { confirmPassword?: string }>();
  const [passwordTarget, setPasswordTarget] = useState<UserSummary>();
  const [passwordForm] = Form.useForm<{ password: string; confirmPassword: string }>();

  const save = useMutation({
    mutationFn: (values: UserWriteValues) =>
      editing === 'create'
        ? administrationAPI.users.create(values)
        : administrationAPI.users.update((editing as UserSummary).id, values),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: queryKeys.administration('users') });
      setEditing(undefined);
      form.resetFields();
      finishManagementRouteIntent(routeIntent, '/users');
      void message.success(intl.formatMessage({ id: 'administration.save.success' }));
    },
  });
  const remove = useMutation({
    mutationFn: (user: UserSummary) => administrationAPI.users.remove(user.id),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: queryKeys.administration('users') });
      void message.success(intl.formatMessage({ id: 'administration.delete.success' }));
    },
  });
  const resetPassword = useMutation({
    mutationFn: ({ id, password }: { id: string; password: string }) =>
      administrationAPI.users.resetPassword(id, password),
    onSuccess: () => {
      passwordForm.resetFields();
      setPasswordTarget(undefined);
      finishManagementRouteIntent(routeIntent, '/users');
      void message.success(intl.formatMessage({ id: 'user.passwordReset.success' }));
    },
  });

  const openEditor = (user: UserSummary | 'create') => {
    setEditing(user);
    save.reset();
    if (user === 'create') {
      form.setFieldsValue({ status: 'enabled' });
      return;
    }
    form.setFieldsValue({
      username: user.username,
      name: user.name,
      email: user.email,
      roleID: user.roleID || user.role?.id,
      departmentID: user.departmentID || user.department?.id,
      postID: user.postID || user.post?.id,
      status: user.status,
      password: undefined,
      confirmPassword: undefined,
    });
  };

  useManagementRouteIntent(routeIntent, {
    load: administrationAPI.users.get,
    openCreate: () => openEditor('create'),
    openEdit: openEditor,
    openResetPassword: (user) => {
      passwordForm.resetFields();
      setPasswordTarget(user);
    },
    onError: (error) => {
      void message.error(getRequestErrorMessage(error));
      finishManagementRouteIntent(routeIntent, '/users');
    },
  });

  const compiledColumns: TableColumnsType<UserSummary> = [
    {
      title: intl.formatMessage({ id: 'user.field.account' }),
      dataIndex: 'username',
      width: 210,
      render: (_, user) => (
        <Space>
          <Avatar src={user.avatar || undefined}>
            {(user.name || user.username).slice(0, 1).toUpperCase()}
          </Avatar>
          <span>{user.username}</span>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({ id: 'administration.field.name' }),
      dataIndex: 'name',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'user.field.email' }),
      dataIndex: 'email',
      ellipsis: true,
      responsive: ['md'],
    },
    {
      title: intl.formatMessage({ id: 'user.field.role' }),
      key: 'roleName',
      width: 150,
      render: (_, user) => administrationReferenceName(user.role, user.roleID, roleNamesByID),
    },
    {
      title: intl.formatMessage({ id: 'user.field.organization' }),
      key: 'organization',
      responsive: ['lg'],
      render: (_, user) => {
        const department = administrationReferenceName(
          user.department,
          user.departmentID,
          departmentNamesByID,
        );
        const post = administrationReferenceName(user.post, user.postID, postNamesByID);
        return [department, post].filter((name) => name !== '—').join(' · ') || '—';
      },
    },
    {
      title: intl.formatMessage({ id: 'administration.field.status' }),
      dataIndex: 'status',
      width: 120,
      render: (_, user) => <AdministrationStatusTag status={user.status} />,
    },
    {
      title: intl.formatMessage({ id: 'administration.field.actions' }),
      key: 'actions',
      width: 300,
      fixed: 'right',
      render: (_, user) => {
        const rootTarget = (user.role ?? rolesByID.get(user.roleID))?.root === true;
        return (
          <Space size="small" wrap>
            {canEdit ? (
              <Button
                disabled={rootTarget}
                size="small"
                type="link"
                onClick={() => openEditor(user)}
              >
                {intl.formatMessage({ id: 'actions.edit' })}
              </Button>
            ) : null}
            {canResetPassword ? (
              <Button
                size="small"
                type="link"
                onClick={() => {
                  passwordForm.resetFields();
                  setPasswordTarget(user);
                }}
              >
                {intl.formatMessage({ id: 'user.passwordReset.action' })}
              </Button>
            ) : null}
            {canDelete ? (
              <Popconfirm
                description={intl.formatMessage({ id: 'user.delete.description' })}
                disabled={rootTarget}
                title={intl.formatMessage({ id: 'user.delete.confirm' })}
                onConfirm={() => remove.mutate(user)}
              >
                <Button danger disabled={rootTarget} size="small" type="link">
                  {intl.formatMessage({ id: 'actions.delete' })}
                </Button>
              </Popconfirm>
            ) : null}
          </Space>
        );
      },
    },
  ];

  const compiledColumnByField = new Map<string, TableColumnsType<UserSummary>[number]>();
  let compiledActionColumn: TableColumnsType<UserSummary>[number] | undefined;
  for (const column of compiledColumns) {
    if ('dataIndex' in column && typeof column.dataIndex === 'string') {
      compiledColumnByField.set(column.dataIndex, column);
    } else if ('key' in column && typeof column.key === 'string') {
      if (column.key === 'actions') compiledActionColumn = column;
      else compiledColumnByField.set(column.key, column);
    }
  }
  const columns: TableColumnsType<UserSummary> = presentation.list.columns.flatMap((field) => {
    const expectedComponent =
      userPresentationListComponents[field.field as keyof typeof userPresentationListComponents];
    const column = compiledColumnByField.get(field.field);
    if (!column || !expectedComponent || field.component !== expectedComponent) return [];
    return [
      {
        ...column,
        title: field.label,
        ...(field.width !== undefined ? { width: field.width } : {}),
      },
    ];
  });
  if (compiledActionColumn) columns.push(compiledActionColumn);
  const mobileFields = new Set<string>(userPresentationMobileFields);
  const mobileColumnKeys = presentation.list.columns
    .map((field) => field.field)
    .filter((field) => mobileFields.has(field));
  if (compiledActionColumn) mobileColumnKeys.push('actions');
  const nameSearch =
    presentation.search.fields.find(
      (field) =>
        field.field === 'name' && field.component === userPresentationSearchComponents.name,
    ) ?? null;
  const statusSearch =
    presentation.search.fields.find(
      (field) =>
        field.field === 'status' && field.component === userPresentationSearchComponents.status,
    ) ?? null;

  const dependencyError = roles.error || departments.error || posts.error;

  return (
    <>
      <AdministrationTable
        columns={columns}
        density={presentation.list.density}
        emptyText={intl.formatMessage({ id: 'user.empty' })}
        nameSearch={nameSearch}
        params={params}
        query={users}
        resetPageSize={configuredPageSize}
        searchCollapsedByDefault={presentation.search.collapsedByDefault}
        setParams={updateParams}
        statusSearch={statusSearch}
        mobileColumnKeys={mobileColumnKeys}
        toolbar={
          canCreate ? (
            <Button type="primary" onClick={() => openEditor('create')}>
              {intl.formatMessage({ id: 'user.create.action' })}
            </Button>
          ) : null
        }
      />
      <Modal
        destroyOnHidden
        forceRender
        confirmLoading={save.isPending}
        okButtonProps={{
          disabled:
            roles.isPending ||
            roles.isError ||
            departments.isPending ||
            departments.isError ||
            posts.isPending ||
            posts.isError,
        }}
        open={Boolean(editing)}
        title={intl.formatMessage({
          id: editing === 'create' ? 'user.create.title' : 'user.edit.title',
        })}
        width={720}
        onCancel={() => {
          setEditing(undefined);
          form.resetFields();
          save.reset();
          finishManagementRouteIntent(routeIntent, '/users');
        }}
        onOk={() => form.submit()}
      >
        {dependencyError ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(dependencyError)}
            showIcon
            title={intl.formatMessage({ id: 'user.dependencies.failed' })}
            type="error"
          />
        ) : null}
        {save.isError ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(save.error)}
            showIcon
            type="error"
          />
        ) : null}
        <Form<UserWriteValues & { confirmPassword?: string }>
          form={form}
          layout="vertical"
          onFinish={(values) => save.mutate(values)}
        >
          <Form.Item
            name="username"
            label={intl.formatMessage({ id: 'user.field.username' })}
            rules={[{ required: true }, { min: 3 }, { max: 20 }, { pattern: /^[A-Za-z0-9_]+$/ }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="name"
            label={intl.formatMessage({ id: 'administration.field.name' })}
            rules={[{ max: 100 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="email"
            label={intl.formatMessage({ id: 'user.field.email' })}
            rules={[{ type: 'email' }, { max: 100 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          {editing === 'create' ? (
            <>
              <Form.Item
                name="password"
                label={intl.formatMessage({ id: 'user.field.password' })}
                rules={[
                  { required: true },
                  { min: 8 },
                  { max: 128 },
                  { pattern: /[A-Za-z]/ },
                  { pattern: /\d/ },
                ]}
              >
                <Input.Password autoComplete="new-password" />
              </Form.Item>
              <Form.Item
                dependencies={['password']}
                name="confirmPassword"
                label={intl.formatMessage({ id: 'user.field.confirmPassword' })}
                rules={[
                  { required: true },
                  ({ getFieldValue }) => ({
                    validator: async (_, value) => {
                      if (value === getFieldValue('password')) return;
                      throw new Error(intl.formatMessage({ id: 'user.password.mismatch' }));
                    },
                  }),
                ]}
              >
                <Input.Password autoComplete="new-password" />
              </Form.Item>
            </>
          ) : null}
          <Form.Item
            name="roleID"
            label={intl.formatMessage({ id: 'user.field.role' })}
            rules={[{ required: true }]}
          >
            <Select
              disabled={roles.isError}
              loading={roles.isPending}
              options={(roles.data ?? []).map((role) => ({
                value: role.id,
                label: role.name,
                disabled: role.status !== 'enabled',
              }))}
            />
          </Form.Item>
          <Form.Item
            name="departmentID"
            label={intl.formatMessage({ id: 'user.field.department' })}
          >
            <Select
              allowClear
              showSearch
              disabled={departments.isError}
              loading={departments.isPending}
              optionFilterProp="label"
              options={departmentOptions}
            />
          </Form.Item>
          <Form.Item name="postID" label={intl.formatMessage({ id: 'user.field.post' })}>
            <Select
              allowClear
              showSearch
              disabled={posts.isError}
              loading={posts.isPending}
              optionFilterProp="label"
              options={postOptions}
            />
          </Form.Item>
          <Form.Item
            name="status"
            label={intl.formatMessage({ id: 'administration.field.status' })}
            rules={[{ required: true }]}
          >
            <Select
              options={(['enabled', 'disabled', 'locked'] as const).map((value) => ({
                value,
                label: intl.formatMessage({ id: `administration.status.${value}` }),
              }))}
            />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        destroyOnHidden
        forceRender
        confirmLoading={resetPassword.isPending}
        open={Boolean(passwordTarget)}
        title={intl.formatMessage(
          { id: 'user.passwordReset.title' },
          { username: passwordTarget?.username ?? '' },
        )}
        onCancel={() => {
          passwordForm.resetFields();
          setPasswordTarget(undefined);
          resetPassword.reset();
          finishManagementRouteIntent(routeIntent, '/users');
        }}
        onOk={() => passwordForm.submit()}
      >
        <Alert
          className="mb-4"
          description={intl.formatMessage({ id: 'user.passwordReset.securityNotice' })}
          showIcon
          type="warning"
        />
        {resetPassword.isError ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(resetPassword.error)}
            showIcon
            type="error"
          />
        ) : null}
        <Form
          form={passwordForm}
          layout="vertical"
          onFinish={(values) => {
            if (passwordTarget)
              resetPassword.mutate({ id: passwordTarget.id, password: values.password });
          }}
        >
          <Form.Item
            name="password"
            label={intl.formatMessage({ id: 'user.field.password' })}
            rules={[
              { required: true },
              { min: 8 },
              { max: 128 },
              { pattern: /[A-Za-z]/ },
              { pattern: /\d/ },
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            dependencies={['password']}
            name="confirmPassword"
            label={intl.formatMessage({ id: 'user.field.confirmPassword' })}
            rules={[
              { required: true },
              ({ getFieldValue }) => ({
                validator: async (_, value) => {
                  if (value === getFieldValue('password')) return;
                  throw new Error(intl.formatMessage({ id: 'user.password.mismatch' }));
                },
              }),
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
