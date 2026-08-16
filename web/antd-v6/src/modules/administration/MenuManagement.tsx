import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import type { TableColumnsType } from 'antd';
import {
  Alert,
  App,
  Button,
  Checkbox,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Tag,
} from 'antd';
import { useMemo, useState } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { formatMenuLabel } from '@/shared/navigation/menuLocale';
import { queryKeys } from '@/shared/query/client';
import AdministrationTable, { AdministrationStatusTag } from './AdministrationTable';
import { administrationAPI } from './api';
import {
  type AdministrationListParams,
  administrationSelectOptions,
  flattenAdministrationTree,
  type MenuSummary,
  type MenuWriteValues,
} from './contract';
import { useAdministrationPage } from './query';

interface MenuManagementProps {
  canCreate: boolean;
  canDelete: boolean;
  canEdit: boolean;
}

const initialParams: AdministrationListParams = {
  current: 1,
  pageSize: 20,
  status: 'all',
};

export default function MenuManagement({ canCreate, canDelete, canEdit }: MenuManagementProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const [params, setParams] = useState(initialParams);
  const menus = useAdministrationPage('menus', params);
  const [editing, setEditing] = useState<MenuSummary | 'create'>();
  const [form] = Form.useForm<MenuWriteValues>();
  const selectedType = Form.useWatch('type', form);

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

  const disabledParents = useMemo(() => {
    if (!editing || editing === 'create') return new Set<string>();
    return new Set(flattenAdministrationTree([editing]).map((item) => item.id));
  }, [editing]);
  const parentOptions = useMemo(
    () =>
      administrationSelectOptions(menus.data?.data ?? [], {
        disabled: disabledParents,
        include: (menu) => menu.type !== 'COMPONENT' && menu.type !== 'API',
      }),
    [disabledParents, menus.data?.data],
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
      width: 190,
      fixed: 'right',
      render: (_, menu) => (
        <Space size="small">
          {canEdit ? (
            <Button size="small" type="link" onClick={() => openEditor(menu)}>
              {intl.formatMessage({ id: 'actions.edit' })}
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
        open={Boolean(editing)}
        title={intl.formatMessage({
          id: editing === 'create' ? 'menu.create.title' : 'menu.edit.title',
        })}
        width={720}
        onCancel={() => {
          setEditing(undefined);
          form.resetFields();
          save.reset();
        }}
        onOk={() => form.submit()}
      >
        <Alert
          className="mb-4"
          description={intl.formatMessage({ id: 'menu.compiledRegistry.notice' })}
          showIcon
          type="info"
        />
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
            <Select allowClear showSearch optionFilterProp="label" options={parentOptions} />
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
            <InputNumber className="w-full" min={-1_000_000} max={1_000_000} precision={0} />
          </Form.Item>
          <Form.Item name="hideInMenu" valuePropName="checked">
            <Checkbox>{intl.formatMessage({ id: 'menu.field.hideInMenu' })}</Checkbox>
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
