import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import type { TableColumnsType } from 'antd';
import {
  Alert,
  App,
  Button,
  Checkbox,
  Drawer,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Tag,
  Transfer,
} from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import {
  finishManagementRouteIntent,
  type ManagementRouteIntent,
  useManagementRouteIntent,
} from '@/shared/navigation/managementRoute';
import { formatMenuLabel } from '@/shared/navigation/menuLocale';
import { queryKeys } from '@/shared/query/client';
import AdministrationTable, { AdministrationStatusTag } from './AdministrationTable';
import { administrationAPI } from './api';
import {
  type AdministrationListParams,
  administrationAPIReferenceKey,
  administrationSelectOptions,
  administrationSubtreeIDs,
  type MenuSummary,
  type MenuWriteValues,
} from './contract';
import {
  useAdministrationAPICatalog,
  useAdministrationPage,
  useMenuAPIBindings,
  useMenuTree,
} from './query';

interface MenuManagementProps {
  canBindAPI: boolean;
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

export default function MenuManagement({
  canBindAPI,
  canCreate,
  canDelete,
  canEdit,
  routeIntent,
}: MenuManagementProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const [params, setParams] = useState(initialParams);
  const menus = useAdministrationPage('menus', params);
  const menuCatalog = useMenuTree();
  const [editing, setEditing] = useState<MenuSummary | 'create'>();
  const [bindingMenu, setBindingMenu] = useState<MenuSummary>();
  const [boundAPIKeys, setBoundAPIKeys] = useState<string[]>([]);
  const [form] = Form.useForm<MenuWriteValues>();
  const selectedType = Form.useWatch('type', form);
  const apiCatalog = useAdministrationAPICatalog(Boolean(bindingMenu));
  const boundAPIs = useMenuAPIBindings(bindingMenu?.id);

  useEffect(() => {
    if (!boundAPIs.data) return;
    setBoundAPIKeys(boundAPIs.data.map(administrationAPIReferenceKey));
  }, [boundAPIs.data]);

  const save = useMutation({
    mutationFn: (values: MenuWriteValues) =>
      editing === 'create'
        ? administrationAPI.menus.create(values)
        : administrationAPI.menus.update((editing as MenuSummary).id, values),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: queryKeys.administration('menus') }),
        client.invalidateQueries({ queryKey: ['authorization'] }),
      ]);
      setEditing(undefined);
      form.resetFields();
      finishManagementRouteIntent(routeIntent, '/menu');
      void message.success(intl.formatMessage({ id: 'administration.save.success' }));
    },
  });
  const remove = useMutation({
    mutationFn: (menu: MenuSummary) => administrationAPI.menus.remove(menu.id),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: queryKeys.administration('menus') }),
        client.invalidateQueries({ queryKey: ['authorization'] }),
      ]);
      void message.success(intl.formatMessage({ id: 'administration.delete.success' }));
    },
  });
  const bindAPIs = useMutation({
    mutationFn: async () => {
      if (!bindingMenu) throw new Error('Menu API binding target is unavailable');
      await administrationAPI.menus.bindAPIs(bindingMenu.id, boundAPIKeys);
    },
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({
          queryKey: ['administration', 'menus', bindingMenu?.id ?? '', 'apis'],
        }),
        client.invalidateQueries({ queryKey: ['authorization'] }),
      ]);
      setBindingMenu(undefined);
      setBoundAPIKeys([]);
      void message.success(intl.formatMessage({ id: 'menu.apiBinding.success' }));
    },
  });

  const disabledParents = useMemo(() => {
    if (!editing || editing === 'create') return new Set<string>();
    return administrationSubtreeIDs(menuCatalog.data ?? [], editing.id);
  }, [editing, menuCatalog.data]);
  const parentOptions = useMemo(
    () =>
      administrationSelectOptions(menuCatalog.data ?? [], {
        disabled: disabledParents,
        include: (menu) => menu.type !== 'COMPONENT' && menu.type !== 'API',
        label: (menu) => formatMenuLabel(intl, menu.name),
      }),
    [disabledParents, intl, menuCatalog.data],
  );

  const openEditor = (menu: MenuSummary | 'create') => {
    setEditing(menu);
    save.reset();
    if (menu === 'create') {
      form.setFieldsValue({
        type: 'MENU',
        method: 'GET',
        status: 'enabled',
        sort: 0,
        hideInMenu: false,
      });
      return;
    }
    form.setFieldsValue({
      parentID: menu.parentID,
      name: menu.name,
      path: menu.path,
      permission: menu.permission,
      method: menu.method,
      type: menu.type,
      component: menu.component,
      icon: menu.icon,
      hideInMenu: menu.hideInMenu,
      status: menu.status,
      sort: menu.sort,
    });
  };

  useManagementRouteIntent(routeIntent, {
    load: administrationAPI.menus.get,
    openCreate: () => openEditor('create'),
    openEdit: openEditor,
    onError: (error) => {
      void message.error(getRequestErrorMessage(error));
      finishManagementRouteIntent(routeIntent, '/menu');
    },
  });

  const columns: TableColumnsType<MenuSummary> = [
    {
      title: intl.formatMessage({ id: 'administration.field.name' }),
      dataIndex: 'name',
      width: 240,
      render: (_, menu) => formatMenuLabel(intl, menu.name),
    },
    {
      title: intl.formatMessage({ id: 'menu.field.path' }),
      dataIndex: 'path',
      ellipsis: true,
      render: (_, menu) => <code>{menu.path || '—'}</code>,
    },
    {
      title: intl.formatMessage({ id: 'menu.field.type' }),
      dataIndex: 'type',
      width: 125,
      render: (_, menu) => <Tag>{menu.type}</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'menu.field.permission' }),
      dataIndex: 'permission',
      ellipsis: true,
      responsive: ['lg'],
      render: (_, menu) => menu.permission || '—',
    },
    {
      title: intl.formatMessage({ id: 'administration.field.status' }),
      dataIndex: 'status',
      width: 120,
      render: (_, menu) => <AdministrationStatusTag status={menu.status} />,
    },
    {
      title: intl.formatMessage({ id: 'administration.field.actions' }),
      key: 'actions',
      width: 280,
      fixed: 'right',
      render: (_, menu) => (
        <Space size="small">
          {canEdit ? (
            <Button size="small" type="link" onClick={() => openEditor(menu)}>
              {intl.formatMessage({ id: 'actions.edit' })}
            </Button>
          ) : null}
          {canBindAPI && menu.type !== 'DIRECTORY' && menu.type !== 'API' ? (
            <Button
              size="small"
              type="link"
              onClick={() => {
                bindAPIs.reset();
                setBoundAPIKeys([]);
                setBindingMenu(menu);
              }}
            >
              {intl.formatMessage({ id: 'menu.apiBinding.action' })}
            </Button>
          ) : null}
          {canDelete ? (
            <Popconfirm
              description={intl.formatMessage({ id: 'menu.delete.description' })}
              title={intl.formatMessage({ id: 'menu.delete.confirm' })}
              onConfirm={() => remove.mutate(menu)}
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
        emptyText={intl.formatMessage({ id: 'menu.empty' })}
        params={params}
        query={menus}
        setParams={setParams}
        mobileColumnKeys={['name', 'type', 'path', 'status', 'actions']}
        toolbar={
          canCreate ? (
            <Button type="primary" onClick={() => openEditor('create')}>
              {intl.formatMessage({ id: 'menu.create.action' })}
            </Button>
          ) : null
        }
      />
      <Modal
        destroyOnHidden
        forceRender
        confirmLoading={save.isPending}
        okButtonProps={{ disabled: menuCatalog.isPending || menuCatalog.isError }}
        open={Boolean(editing)}
        title={intl.formatMessage({
          id: editing === 'create' ? 'menu.create.title' : 'menu.edit.title',
        })}
        width={720}
        onCancel={() => {
          setEditing(undefined);
          form.resetFields();
          save.reset();
          finishManagementRouteIntent(routeIntent, '/menu');
        }}
        onOk={() => form.submit()}
      >
        <Alert
          className="mb-4"
          description={intl.formatMessage({ id: 'menu.compiledRegistry.notice' })}
          showIcon
          type="info"
        />
        {menuCatalog.isError ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(menuCatalog.error)}
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
        <Form<MenuWriteValues>
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
              disabled={menuCatalog.isError}
              loading={menuCatalog.isPending}
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
            name="type"
            label={intl.formatMessage({ id: 'menu.field.type' })}
            rules={[{ required: true }]}
          >
            <Select
              options={(['DIRECTORY', 'MENU', 'COMPONENT'] as const).map((value) => ({
                value,
                label: value,
              }))}
            />
          </Form.Item>
          <Form.Item
            name="path"
            label={intl.formatMessage({ id: 'menu.field.path' })}
            rules={[{ required: true }, { max: 255 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          {selectedType !== 'DIRECTORY' ? (
            <Form.Item
              name="permission"
              label={intl.formatMessage({ id: 'menu.field.permission' })}
              rules={[{ max: 255 }]}
            >
              <Input autoComplete="off" />
            </Form.Item>
          ) : null}
          <Form.Item
            name="component"
            label={intl.formatMessage({ id: 'menu.field.component' })}
            rules={[{ max: 255 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="icon"
            label={intl.formatMessage({ id: 'menu.field.icon' })}
            rules={[{ max: 255 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="method"
            label={intl.formatMessage({ id: 'menu.field.method' })}
            rules={[{ required: true }]}
          >
            <Select
              options={['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map((value) => ({
                value,
                label: value,
              }))}
            />
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
          <Form.Item name="hideInMenu" valuePropName="checked">
            <Checkbox>{intl.formatMessage({ id: 'menu.field.hideInMenu' })}</Checkbox>
          </Form.Item>
        </Form>
      </Modal>
      <Drawer
        destroyOnHidden
        open={Boolean(bindingMenu)}
        title={intl.formatMessage(
          { id: 'menu.apiBinding.title' },
          { name: bindingMenu ? formatMenuLabel(intl, bindingMenu.name) : '' },
        )}
        width={760}
        extra={
          <Button
            type="primary"
            loading={bindAPIs.isPending}
            disabled={
              apiCatalog.isPending || boundAPIs.isPending || apiCatalog.isError || boundAPIs.isError
            }
            onClick={() => bindAPIs.mutate()}
          >
            {intl.formatMessage({ id: 'actions.save' })}
          </Button>
        }
        onClose={() => {
          setBindingMenu(undefined);
          setBoundAPIKeys([]);
          bindAPIs.reset();
        }}
      >
        {apiCatalog.isError || boundAPIs.isError || bindAPIs.isError ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(
              apiCatalog.error ?? boundAPIs.error ?? bindAPIs.error,
            )}
            showIcon
            type="error"
          />
        ) : null}
        <Alert
          className="mb-4"
          description={intl.formatMessage({ id: 'menu.apiBinding.description' })}
          showIcon
          type="info"
        />
        <Transfer
          dataSource={(apiCatalog.data ?? []).map((api) => ({
            key: administrationAPIReferenceKey(api),
            title: `${api.method} ${api.path}`,
            description: api.name,
          }))}
          disabled={apiCatalog.isPending || boundAPIs.isPending}
          styles={{ section: { width: 'calc(50% - 24px)', height: 480 } }}
          render={(item) => item.title}
          showSearch
          targetKeys={boundAPIKeys}
          titles={[
            intl.formatMessage({ id: 'menu.apiBinding.available' }),
            intl.formatMessage({ id: 'menu.apiBinding.bound' }),
          ]}
          onChange={(keys) => setBoundAPIKeys(keys.map(String))}
        />
      </Drawer>
    </>
  );
}
