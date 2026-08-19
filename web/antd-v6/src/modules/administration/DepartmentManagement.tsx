import { getRequestErrorMessage, getRequestStatus } from '@mss-admin-core/shared/api/errors';
import {
  finishManagementRouteIntent,
  type ManagementRouteIntent,
  useManagementRouteIntent,
} from '@mss-admin-core/shared/navigation/managementRoute';
import { queryKeys } from '@mss-admin-core/shared/query/client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import type { TableColumnsType } from 'antd';
import {
  Alert,
  App,
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
} from 'antd';
import { useMemo, useState } from 'react';
import AdministrationTable, { AdministrationStatusTag } from './AdministrationTable';
import { administrationAPI } from './api';
import {
  type AdministrationListParams,
  administrationReferenceName,
  administrationSelectOptions,
  administrationSubtreeIDs,
  type DepartmentSummary,
  type DepartmentWriteValues,
} from './contract';
import { useAdministrationCatalog, useAdministrationPage } from './query';

interface DepartmentManagementProps {
  canCreate: boolean;
  canDelete: boolean;
  canEdit: boolean;
  routeIntent?: ManagementRouteIntent;
}

const initialParams: AdministrationListParams = {
  current: 1,
  pageSize: 20,
  status: 'all',
};

export default function DepartmentManagement({
  canCreate,
  canDelete,
  canEdit,
  routeIntent,
}: DepartmentManagementProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const [params, setParams] = useState(initialParams);
  const departments = useAdministrationPage('departments', params);
  const departmentCatalog = useAdministrationCatalog('departments');
  const users = useAdministrationCatalog('users');
  const userNamesByID = useMemo(
    () => new Map((users.data ?? []).map((user) => [user.id, user.name || user.username] as const)),
    [users.data],
  );
  const userOptions = useMemo(
    () =>
      (users.data ?? []).map((user) => ({
        disabled: user.status !== 'enabled',
        label: user.name ? `${user.name} (${user.username})` : user.username,
        value: user.id,
      })),
    [users.data],
  );
  const [editing, setEditing] = useState<DepartmentSummary | 'create'>();
  const [form] = Form.useForm<DepartmentWriteValues>();

  const save = useMutation({
    mutationFn: (values: DepartmentWriteValues) =>
      editing === 'create'
        ? administrationAPI.departments.create(values)
        : administrationAPI.departments.update((editing as DepartmentSummary).id, values),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: queryKeys.administration('departments') });
      setEditing(undefined);
      form.resetFields();
      finishManagementRouteIntent(routeIntent, '/departments');
      void message.success(intl.formatMessage({ id: 'administration.save.success' }));
    },
  });
  const remove = useMutation({
    mutationFn: (department: DepartmentSummary) =>
      administrationAPI.departments.remove(department.id),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: queryKeys.administration('departments') });
      void message.success(intl.formatMessage({ id: 'administration.delete.success' }));
    },
  });

  const disabledParents = useMemo(() => {
    if (!editing || editing === 'create') return new Set<string>();
    return administrationSubtreeIDs(departmentCatalog.data ?? [], editing.id);
  }, [departmentCatalog.data, editing]);
  const parentOptions = useMemo(
    () => administrationSelectOptions(departmentCatalog.data ?? [], { disabled: disabledParents }),
    [departmentCatalog.data, disabledParents],
  );

  const openEditor = (department: DepartmentSummary | 'create') => {
    setEditing(department);
    save.reset();
    if (department === 'create') {
      form.setFieldsValue({ status: 'enabled', sort: 0 });
      return;
    }
    form.setFieldsValue({
      parentID: department.parentID,
      name: department.name,
      code: department.code,
      leaderID: department.leaderID,
      phone: department.phone,
      email: department.email,
      status: department.status,
      sort: department.sort,
    });
  };

  useManagementRouteIntent(routeIntent, {
    load: administrationAPI.departments.get,
    openCreate: () => openEditor('create'),
    openEdit: openEditor,
    onError: (error) => {
      void message.error(getRequestErrorMessage(error));
      finishManagementRouteIntent(routeIntent, '/departments');
    },
  });

  const columns: TableColumnsType<DepartmentSummary> = [
    {
      title: intl.formatMessage({ id: 'administration.field.name' }),
      dataIndex: 'name',
      width: 240,
    },
    {
      title: intl.formatMessage({ id: 'department.field.code' }),
      dataIndex: 'code',
      width: 160,
    },
    {
      title: intl.formatMessage({ id: 'department.field.leader' }),
      dataIndex: 'leaderID',
      responsive: ['md'],
      width: 150,
      render: (leaderID: string) => administrationReferenceName(undefined, leaderID, userNamesByID),
    },
    {
      title: intl.formatMessage({ id: 'department.field.contact' }),
      key: 'contact',
      responsive: ['lg'],
      render: (_, department) =>
        [department.phone, department.email].filter(Boolean).join(' · ') || '—',
    },
    {
      title: intl.formatMessage({ id: 'administration.field.status' }),
      dataIndex: 'status',
      width: 120,
      render: (_, department) => <AdministrationStatusTag status={department.status} />,
    },
    {
      title: intl.formatMessage({ id: 'administration.field.sort' }),
      dataIndex: 'sort',
      width: 90,
      responsive: ['md'],
    },
    {
      title: intl.formatMessage({ id: 'administration.field.actions' }),
      key: 'actions',
      width: 190,
      fixed: 'right',
      render: (_, department) => (
        <Space size="small">
          {canEdit ? (
            <Button size="small" type="link" onClick={() => openEditor(department)}>
              {intl.formatMessage({ id: 'actions.edit' })}
            </Button>
          ) : null}
          {canDelete ? (
            <Popconfirm
              description={intl.formatMessage({ id: 'department.delete.description' })}
              title={intl.formatMessage({ id: 'department.delete.confirm' })}
              onConfirm={() => remove.mutate(department)}
            >
              <Button danger size="small" type="link">
                {intl.formatMessage({ id: 'actions.delete' })}
              </Button>
            </Popconfirm>
          ) : null}
        </Space>
      ),
    },
  ];

  return (
    <>
      <AdministrationTable
        columns={columns}
        emptyText={intl.formatMessage({ id: 'department.empty' })}
        params={params}
        query={departments}
        setParams={setParams}
        mobileColumnKeys={['name', 'code', 'leaderID', 'status', 'actions']}
        toolbar={
          canCreate ? (
            <Button type="primary" onClick={() => openEditor('create')}>
              {intl.formatMessage({ id: 'department.create.action' })}
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
            users.isPending ||
            users.isError ||
            departmentCatalog.isPending ||
            departmentCatalog.isError,
        }}
        open={Boolean(editing)}
        title={intl.formatMessage({
          id: editing === 'create' ? 'department.create.title' : 'department.edit.title',
        })}
        width={680}
        onCancel={() => {
          setEditing(undefined);
          form.resetFields();
          save.reset();
          finishManagementRouteIntent(routeIntent, '/departments');
        }}
        onOk={() => form.submit()}
      >
        {users.isError || departmentCatalog.isError ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(users.error ?? departmentCatalog.error)}
            showIcon
            title={intl.formatMessage({ id: 'administration.dependencies.failed' })}
            type="error"
          />
        ) : null}
        {save.isError ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(save.error)}
            showIcon
            type={getRequestStatus(save.error) === 409 ? 'warning' : 'error'}
          />
        ) : null}
        <Form<DepartmentWriteValues>
          form={form}
          layout="vertical"
          onFinish={(values) => save.mutate(values)}
        >
          <Form.Item
            name="parentID"
            label={intl.formatMessage({ id: 'administration.field.parent' })}
          >
            <Select
              allowClear
              showSearch
              disabled={departmentCatalog.isError}
              loading={departmentCatalog.isPending}
              optionFilterProp="label"
              options={parentOptions}
            />
          </Form.Item>
          <Form.Item
            name="name"
            label={intl.formatMessage({ id: 'administration.field.name' })}
            rules={[{ required: true }, { max: 255 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="code"
            label={intl.formatMessage({ id: 'department.field.code' })}
            rules={[{ required: true }, { max: 255 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item name="leaderID" label={intl.formatMessage({ id: 'department.field.leader' })}>
            <Select
              allowClear
              disabled={users.isError}
              loading={users.isPending}
              options={userOptions}
              optionFilterProp="label"
              showSearch
            />
          </Form.Item>
          <Form.Item
            name="phone"
            label={intl.formatMessage({ id: 'department.field.phone' })}
            rules={[{ max: 50 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="email"
            label={intl.formatMessage({ id: 'department.field.email' })}
            rules={[{ type: 'email' }, { max: 255 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="status"
            label={intl.formatMessage({ id: 'administration.field.status' })}
            rules={[{ required: true }]}
          >
            <Select
              options={(['enabled', 'disabled'] as const).map((value) => ({
                value,
                label: intl.formatMessage({ id: `administration.status.${value}` }),
              }))}
            />
          </Form.Item>
          <Form.Item name="sort" label={intl.formatMessage({ id: 'administration.field.sort' })}>
            <InputNumber
              className="w-full"
              controls={false}
              min={-1_000_000}
              max={1_000_000}
              precision={0}
            />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
