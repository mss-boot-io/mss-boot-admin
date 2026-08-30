import { getRequestErrorMessage, getRequestStatus } from '@mss-admin-core/shared/api/errors';
import {
  finishManagementRouteIntent,
  type ManagementRouteIntent,
  useManagementRouteIntent,
} from '@mss-admin-core/shared/navigation/managementRoute';
import { formatMenuLabel } from '@mss-admin-core/shared/navigation/menuLocale';
import type { PagePresentationRuntime } from '@mss-admin-core/shared/presentation/runtime';
import {
  resolveTablePresentation,
  usePresentationPageParams,
} from '@mss-admin-core/shared/presentation/table';
import { queryKeys } from '@mss-admin-core/shared/query/client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import type { TableColumnsType } from 'antd';
import {
  Alert,
  App,
  Button,
  Drawer,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Tag,
  Tree,
} from 'antd';
import type { DataNode } from 'antd/es/tree';
import type { Key } from 'react';
import { useEffect, useMemo, useState } from 'react';
import AdministrationTable, { AdministrationStatusTag } from './AdministrationTable';
import { administrationAPI, RoleAuthorizationRevisionConflictError } from './api';
import {
  type AdministrationListParams,
  flattenAdministrationTree,
  isAdminPageSize,
  type MenuSummary,
  type RoleSummary,
  type RoleWriteValues,
} from './contract';
import { useAdministrationPage, useMenuTree, useRoleAuthorization } from './query';
import {
  rolePresentationListComponents,
  rolePresentationMobileFields,
  rolePresentationSearchComponents,
} from './tablePresentation';

interface RoleManagementProps {
  canAuthorize: boolean;
  canCreate: boolean;
  canDelete: boolean;
  canEdit: boolean;
  presentationRuntime: PagePresentationRuntime;
  routeIntent?: ManagementRouteIntent;
}

const initialParams: AdministrationListParams = {
  current: 1,
  pageSize: 20,
  status: 'all',
};

function menuTreeData(
  items: readonly MenuSummary[],
  title: (menu: MenuSummary) => string,
): DataNode[] {
  return items.map((menu) => ({
    key: menu.path || menu.id,
    title: title(menu),
    disabled: !menu.path,
    children: menu.children ? menuTreeData(menu.children, title) : undefined,
  }));
}

export default function RoleManagement({
  canAuthorize,
  canCreate,
  canDelete,
  canEdit,
  presentationRuntime,
  routeIntent,
}: RoleManagementProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const presentation = presentationRuntime.model;
  const configuredPageSize = isAdminPageSize(presentation.list.pageSize)
    ? presentation.list.pageSize
    : initialParams.pageSize;
  const [params, setParams] = usePresentationPageParams(initialParams, configuredPageSize);
  const roles = useAdministrationPage('roles', params);
  const [editing, setEditing] = useState<RoleSummary | 'create'>();
  const [form] = Form.useForm<RoleWriteValues>();
  const [authorizationRole, setAuthorizationRole] = useState<RoleSummary>();
  const [checkedPaths, setCheckedPaths] = useState<Key[]>([]);
  const [authorizationConflict, setAuthorizationConflict] = useState(false);
  const authorization = useRoleAuthorization(authorizationRole?.id);
  const menus = useMenuTree(Boolean(authorizationRole));

  useEffect(() => {
    if (authorization.data) {
      setCheckedPaths(authorization.data.paths);
      setAuthorizationConflict(false);
    }
  }, [authorization.data]);

  const save = useMutation({
    mutationFn: (values: RoleWriteValues) =>
      editing === 'create'
        ? administrationAPI.roles.create(values)
        : administrationAPI.roles.update((editing as RoleSummary).id, values),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: queryKeys.administration('roles') });
      setEditing(undefined);
      form.resetFields();
      finishManagementRouteIntent(routeIntent, '/role');
      void message.success(intl.formatMessage({ id: 'administration.save.success' }));
    },
  });
  const remove = useMutation({
    mutationFn: (role: RoleSummary) => administrationAPI.roles.remove(role.id),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: queryKeys.administration('roles') });
      void message.success(intl.formatMessage({ id: 'administration.delete.success' }));
    },
  });
  const saveAuthorization = useMutation({
    mutationFn: async () => {
      if (!authorization.data) throw new Error('Role authorization base is unavailable');
      return administrationAPI.roles.saveAuthorization(
        authorization.data,
        checkedPaths.map(String),
      );
    },
    onSuccess: async (resource) => {
      client.setQueryData(queryKeys.roleAuthorization(resource.roleID), resource);
      await client.invalidateQueries({ queryKey: queryKeys.currentUser });
      setAuthorizationConflict(false);
      void message.success(intl.formatMessage({ id: 'role.authorization.saved' }));
    },
    onError: (error) => {
      if (error instanceof RoleAuthorizationRevisionConflictError) {
        if (error.current && authorizationRole) {
          client.setQueryData(queryKeys.roleAuthorization(authorizationRole.id), error.current);
        }
        setAuthorizationConflict(true);
      }
    },
  });

  const treeData = useMemo(
    () => menuTreeData(menus.data ?? [], (menu) => formatMenuLabel(intl, menu.name)),
    [intl, menus.data],
  );
  const selectablePaths = useMemo(
    () =>
      new Set(
        flattenAdministrationTree(menus.data ?? [])
          .map((menu) => menu.path)
          .filter(Boolean),
      ),
    [menus.data],
  );

  const openEditor = (role: RoleSummary | 'create') => {
    setEditing(role);
    if (role === 'create') {
      form.setFieldsValue({ name: '', remark: '', status: 'enabled' });
      return;
    }
    form.setFieldsValue({ name: role.name, remark: role.remark, status: role.status });
  };

  useManagementRouteIntent(routeIntent, {
    load: administrationAPI.roles.get,
    openCreate: () => openEditor('create'),
    openEdit: openEditor,
    onError: (error) => {
      void message.error(getRequestErrorMessage(error));
      finishManagementRouteIntent(routeIntent, '/role');
    },
  });

  const compiledColumns: TableColumnsType<RoleSummary> = [
    {
      title: intl.formatMessage({ id: 'administration.field.name' }),
      dataIndex: 'name',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'role.field.classification' }),
      key: 'classification',
      render: (_, role) => (
        <Space size="small">
          {role.root ? <Tag color="gold">root</Tag> : null}
          {role.default ? (
            <Tag color="blue">{intl.formatMessage({ id: 'role.default' })}</Tag>
          ) : null}
          {!role.root && !role.default ? (
            <Tag>{intl.formatMessage({ id: 'role.ordinary' })}</Tag>
          ) : null}
        </Space>
      ),
    },
    {
      title: intl.formatMessage({ id: 'administration.field.status' }),
      dataIndex: 'status',
      width: 120,
      render: (_, role) => <AdministrationStatusTag status={role.status} />,
    },
    {
      title: intl.formatMessage({ id: 'administration.field.remark' }),
      dataIndex: 'remark',
      ellipsis: true,
      responsive: ['lg'],
    },
    {
      title: intl.formatMessage({ id: 'administration.field.actions' }),
      key: 'actions',
      width: 300,
      fixed: 'right',
      render: (_, role) => {
        const managed = role.root || role.default;
        return (
          <Space size="small" wrap>
            {canEdit ? (
              <Button disabled={managed} size="small" type="link" onClick={() => openEditor(role)}>
                {intl.formatMessage({ id: 'actions.edit' })}
              </Button>
            ) : null}
            {canAuthorize ? (
              <Button
                disabled={role.root}
                size="small"
                type="link"
                onClick={() => setAuthorizationRole(role)}
              >
                {intl.formatMessage({ id: 'role.authorization.action' })}
              </Button>
            ) : null}
            {canDelete ? (
              <Popconfirm
                description={intl.formatMessage({ id: 'role.delete.description' })}
                disabled={managed}
                title={intl.formatMessage({ id: 'role.delete.confirm' })}
                onConfirm={() => remove.mutate(role)}
              >
                <Button danger disabled={managed} size="small" type="link">
                  {intl.formatMessage({ id: 'actions.delete' })}
                </Button>
              </Popconfirm>
            ) : null}
          </Space>
        );
      },
    },
  ];
  const tablePresentation = resolveTablePresentation({
    compiledColumns,
    fallbackPageSize: initialParams.pageSize,
    isPageSize: isAdminPageSize,
    listComponents: rolePresentationListComponents,
    mobileColumnKeys: [...rolePresentationMobileFields, 'actions'],
    model: presentation,
    protectedColumnKeys: ['actions'],
    searchComponents: rolePresentationSearchComponents,
  });

  return (
    <>
      <AdministrationTable
        columns={tablePresentation.columns}
        density={tablePresentation.density}
        emptyText={intl.formatMessage({ id: 'role.empty' })}
        nameSearch={tablePresentation.searchFields.get('name') ?? null}
        params={params}
        query={roles}
        resetPageSize={tablePresentation.pageSize}
        searchCollapsedByDefault={tablePresentation.searchCollapsedByDefault}
        setParams={setParams}
        statusSearch={tablePresentation.searchFields.get('status') ?? null}
        mobileColumnKeys={tablePresentation.mobileColumnKeys}
        toolbar={
          canCreate ? (
            <Button type="primary" onClick={() => openEditor('create')}>
              {intl.formatMessage({ id: 'role.create.action' })}
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
          id: editing === 'create' ? 'role.create.title' : 'role.edit.title',
        })}
        onCancel={() => {
          setEditing(undefined);
          save.reset();
          finishManagementRouteIntent(routeIntent, '/role');
        }}
        onOk={() => form.submit()}
      >
        {save.isError ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(save.error)}
            showIcon
            type={getRequestStatus(save.error) === 409 ? 'warning' : 'error'}
          />
        ) : null}
        <Form<RoleWriteValues>
          form={form}
          layout="vertical"
          onFinish={(values) => save.mutate(values)}
        >
          <Form.Item
            name="name"
            label={intl.formatMessage({ id: 'administration.field.name' })}
            rules={[{ required: true }, { max: 255 }]}
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
          <Form.Item
            name="remark"
            label={intl.formatMessage({ id: 'administration.field.remark' })}
            rules={[{ max: 4096 }]}
          >
            <Input.TextArea rows={4} showCount maxLength={4096} />
          </Form.Item>
        </Form>
      </Modal>
      <Drawer
        destroyOnHidden
        extra={
          <Button
            disabled={!authorization.data || authorizationConflict}
            loading={saveAuthorization.isPending}
            type="primary"
            onClick={() => saveAuthorization.mutate()}
          >
            {intl.formatMessage({ id: 'actions.save' })}
          </Button>
        }
        open={Boolean(authorizationRole)}
        size="large"
        title={intl.formatMessage(
          { id: 'role.authorization.title' },
          { name: authorizationRole?.name ?? '' },
        )}
        onClose={() => {
          setAuthorizationRole(undefined);
          setAuthorizationConflict(false);
          saveAuthorization.reset();
        }}
      >
        {authorizationConflict ? (
          <Alert
            action={
              <Button size="small" onClick={() => void authorization.refetch()}>
                {intl.formatMessage({ id: 'role.authorization.reload' })}
              </Button>
            }
            className="mb-4"
            description={intl.formatMessage({ id: 'role.authorization.conflict.description' })}
            showIcon
            title={intl.formatMessage({ id: 'role.authorization.conflict.title' })}
            type="warning"
          />
        ) : null}
        {saveAuthorization.isError && !authorizationConflict ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(saveAuthorization.error)}
            showIcon
            type="error"
          />
        ) : null}
        <Tree
          checkable
          blockNode
          checkedKeys={checkedPaths.filter((path) => selectablePaths.has(String(path)))}
          disabled={authorization.isPending || menus.isPending}
          treeData={treeData}
          onCheck={(keys) => setCheckedPaths(Array.isArray(keys) ? keys : keys.checked)}
        />
      </Drawer>
    </>
  );
}
