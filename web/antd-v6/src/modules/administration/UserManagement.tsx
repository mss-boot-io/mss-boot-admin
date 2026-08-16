import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import type { TableColumnsType } from 'antd';
import { Alert, App, Avatar, Button, Form, Input, Modal, Popconfirm, Select, Space } from 'antd';
import { useMemo, useState } from 'react';
import { getRequestErrorMessage } from '@/shared/api/errors';
import { queryKeys } from '@/shared/query/client';
import AdministrationTable, { AdministrationStatusTag } from './AdministrationTable';
import { administrationAPI } from './api';
import {
  type AdministrationListParams,
  administrationReferenceName,
  administrationSelectOptions,
  flattenAdministrationTree,
  type UserSummary,
  type UserWriteValues,
} from './contract';
import { useAdministrationPage } from './query';

interface UserManagementProps {
  canCreate: boolean;
  canDelete: boolean;
  canEdit: boolean;
  canResetPassword: boolean;
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
}: UserManagementProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const [params, setParams] = useState(initialParams);
  const users = useAdministrationPage('users', params);
  const dependencyParams = useMemo<AdministrationListParams>(
    () => ({ current: 1, pageSize: 100, status: 'enabled' }),
    [],
  );
  const roles = useAdministrationPage('roles', dependencyParams);
  const departments = useAdministrationPage('departments', dependencyParams);
  const posts = useAdministrationPage('posts', dependencyParams);
  const rolesByID = useMemo(
    () => new Map((roles.data?.data ?? []).map((role) => [role.id, role])),
    [roles.data?.data],
  );
  const roleNamesByID = useMemo(
    () => new Map((roles.data?.data ?? []).map((role) => [role.id, role.name] as const)),
    [roles.data?.data],
  );
  const departmentNamesByID = useMemo(
    () =>
      new Map(
        flattenAdministrationTree(departments.data?.data ?? []).map((department) => [
          department.id,
          department.name,
        ]),
      ),
    [departments.data?.data],
  );
  const postNamesByID = useMemo(
    () =>
      new Map(
        flattenAdministrationTree(posts.data?.data ?? []).map((post) => [post.id, post.name]),
      ),
    [posts.data?.data],
  );
  const departmentOptions = useMemo(
    () => administrationSelectOptions(departments.data?.data ?? []),
    [departments.data?.data],
  );
  const postOptions = useMemo(
    () => administrationSelectOptions(posts.data?.data ?? []),
    [posts.data?.data],
  );
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

  const columns: TableColumnsType<UserSummary> = [
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
      key: 'role',
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

  const dependencyError = roles.error || departments.error || posts.error;

  return (
    <>
      <AdministrationTable
        columns={columns}
        emptyText={intl.formatMessage({ id: 'user.empty' })}
        params={params}
        query={users}
        setParams={setParams}
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
        open={Boolean(editing)}
        title={intl.formatMessage({
          id: editing === 'create' ? 'user.create.title' : 'user.edit.title',
        })}
        width={720}
        onCancel={() => {
          setEditing(undefined);
          form.resetFields();
          save.reset();
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
              loading={roles.isFetching}
              options={(roles.data?.data ?? []).map((role) => ({
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
            <Select allowClear showSearch optionFilterProp="label" options={departmentOptions} />
          </Form.Item>
          <Form.Item name="postID" label={intl.formatMessage({ id: 'user.field.post' })}>
            <Select allowClear showSearch optionFilterProp="label" options={postOptions} />
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
