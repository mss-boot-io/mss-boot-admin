import DeleteOutlined from '@ant-design/icons/DeleteOutlined';
import EditOutlined from '@ant-design/icons/EditOutlined';
import EyeOutlined from '@ant-design/icons/EyeOutlined';
import LockOutlined from '@ant-design/icons/LockOutlined';
import PlusOutlined from '@ant-design/icons/PlusOutlined';
import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Descriptions,
  Drawer,
  Form,
  Grid,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  type TableColumnsType,
  Tag,
  Typography,
} from 'antd';
import { useEffect, useState } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageEmpty, PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import {
  finishManagementRouteIntent,
  type ManagementRouteIntent,
  useManagementRouteIntent,
} from '@/shared/navigation/managementRoute';
import { queryKeys } from '@/shared/query/client';
import { operationsAPI } from './api';
import {
  isOperationsPageSize,
  MAX_SYSTEM_CONFIG_BYTES,
  OPERATIONS_PAGE_SIZES,
  type OperationsPageSize,
  type SystemConfigFormat,
  type SystemConfigSummary,
  type SystemConfigWriteValues,
  systemConfigFormValues,
} from './contract';
import { useSystemConfig, useSystemConfigPage } from './query';

interface SystemConfigManagementProps {
  routeIntent?: ManagementRouteIntent;
}

export default function SystemConfigManagement({ routeIntent }: SystemConfigManagementProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const screens = Grid.useBreakpoint();
  const [params, setParams] = useState<{ current: number; pageSize: OperationsPageSize }>({
    current: 1,
    pageSize: 20,
  });
  const [viewing, setViewing] = useState<string>();
  const [editing, setEditing] = useState<'create' | SystemConfigSummary>();
  const [form] = Form.useForm<SystemConfigWriteValues>();
  const activeID = editing && editing !== 'create' ? editing.id : viewing;
  const detail = useSystemConfig(activeID);
  const configs = useSystemConfigPage(params);
  const format = Form.useWatch('ext', form);

  useEffect(() => {
    if (!editing || editing === 'create' || !detail.data) return;
    form.setFieldsValue(systemConfigFormValues(detail.data));
  }, [detail.data, editing, form]);

  const clearSensitiveDetail = (id?: string) => {
    if (id) queryClient.removeQueries({ queryKey: queryKeys.systemConfig(id) });
  };
  const closeView = () => {
    clearSensitiveDetail(viewing);
    setViewing(undefined);
  };
  const closeEditor = () => {
    clearSensitiveDetail(editing && editing !== 'create' ? editing.id : undefined);
    setEditing(undefined);
    form.resetFields();
    finishManagementRouteIntent(routeIntent, '/system-config');
  };
  const save = useMutation({
    mutationFn: (values: SystemConfigWriteValues) => {
      if (!editing) throw new Error('system configuration editor is not open');
      return editing === 'create'
        ? operationsAPI.systemConfigs.create(values)
        : operationsAPI.systemConfigs.update(editing.id, values);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.systemConfigs });
      closeEditor();
      void message.success(intl.formatMessage({ id: 'systemConfig.save.success' }));
    },
  });
  const remove = useMutation({
    mutationFn: (id: string) => operationsAPI.systemConfigs.remove(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.systemConfigs });
      void message.success(intl.formatMessage({ id: 'systemConfig.delete.success' }));
    },
  });

  const openCreate = () => {
    form.resetFields();
    form.setFieldsValue({ name: '', ext: 'json', content: '{}', remark: '' });
    save.reset();
    setEditing('create');
  };
  const openEdit = (config: SystemConfigSummary) => {
    form.resetFields();
    save.reset();
    setViewing(undefined);
    setEditing(config);
  };
  useManagementRouteIntent(routeIntent, {
    load: operationsAPI.systemConfigs.get,
    openCreate,
    openEdit,
    onError: (error) => {
      void message.error(getRequestErrorMessage(error));
      finishManagementRouteIntent(routeIntent, '/system-config');
    },
  });
  const validateContent = async (_: unknown, value?: string) => {
    if (new TextEncoder().encode(value ?? '').length > MAX_SYSTEM_CONFIG_BYTES) {
      throw new Error(intl.formatMessage({ id: 'systemConfig.content.tooLarge' }));
    }
    if (format === 'json' && value?.trim()) {
      try {
        JSON.parse(value);
      } catch {
        throw new Error(intl.formatMessage({ id: 'systemConfig.content.invalidJson' }));
      }
    }
  };
  const formatDate = (value: string) =>
    new Intl.DateTimeFormat(intl.locale || 'zh-CN', {
      dateStyle: 'short',
      timeStyle: 'medium',
    }).format(new Date(value));

  const columns: TableColumnsType<SystemConfigSummary> = [
    {
      title: intl.formatMessage({ id: 'systemConfig.field.name' }),
      dataIndex: 'name',
      render: (_: unknown, config) => (
        <Space>
          <Button className="px-0" type="link" onClick={() => setViewing(config.id)}>
            {config.name}
          </Button>
          {config.builtIn ? (
            <Tag icon={<LockOutlined />}>{intl.formatMessage({ id: 'systemConfig.builtIn' })}</Tag>
          ) : null}
        </Space>
      ),
    },
    {
      title: intl.formatMessage({ id: 'systemConfig.field.format' }),
      dataIndex: 'ext',
      width: 100,
      render: (value: SystemConfigFormat) => <Tag>{value.toUpperCase()}</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'systemConfig.field.remark' }),
      dataIndex: 'remark',
      responsive: ['md'],
      ellipsis: true,
      render: (value: string) => value || '—',
    },
    {
      title: intl.formatMessage({ id: 'systemConfig.field.updatedAt' }),
      dataIndex: 'updatedAt',
      responsive: ['lg'],
      width: 190,
      render: (value: string) => formatDate(value),
    },
    {
      title: intl.formatMessage({ id: 'systemConfig.field.actions' }),
      key: 'actions',
      width: 260,
      fixed: screens.md ? 'right' : undefined,
      render: (_: unknown, config) => (
        <Space size="small" wrap>
          <Button
            icon={<EyeOutlined />}
            size="small"
            type="link"
            onClick={() => setViewing(config.id)}
          >
            {intl.formatMessage({ id: 'actions.view' })}
          </Button>
          <Button icon={<EditOutlined />} size="small" type="link" onClick={() => openEdit(config)}>
            {intl.formatMessage({ id: 'actions.edit' })}
          </Button>
          <Popconfirm
            description={intl.formatMessage({ id: 'systemConfig.delete.description' })}
            disabled={config.builtIn}
            title={intl.formatMessage({ id: 'systemConfig.delete.confirm' })}
            onConfirm={() => remove.mutate(config.id)}
          >
            <Button
              danger
              disabled={config.builtIn}
              icon={<DeleteOutlined />}
              loading={remove.isPending && remove.variables === config.id}
              size="small"
              type="link"
            >
              {intl.formatMessage({ id: 'actions.delete' })}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const listStatus = getRequestStatus(configs.error);
  if (listStatus === 403) {
    return <PageForbidden message={intl.formatMessage({ id: 'systemConfig.forbidden' })} />;
  }
  if (configs.isPending && !configs.data) return <PageLoading rows={8} />;
  if (configs.isError && !configs.data) {
    return (
      <PageError
        message={getRequestErrorMessage(configs.error)}
        onRetry={() => void configs.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
        title={intl.formatMessage({ id: 'states.loadError' })}
      />
    );
  }

  const editorError = save.error ?? (editing !== 'create' ? detail.error : undefined);
  return (
    <Space orientation="vertical" size="middle" className="w-full">
      <Alert
        description={intl.formatMessage({ id: 'systemConfig.security.description' })}
        showIcon
        title={intl.formatMessage({ id: 'systemConfig.security.title' })}
        type="info"
      />
      {configs.isError ? (
        <Alert
          showIcon
          title={intl.formatMessage({ id: 'operations.refreshFailed' })}
          type="warning"
        />
      ) : null}
      {remove.isError ? (
        <Alert
          closable
          description={getRequestErrorMessage(remove.error)}
          showIcon
          title={intl.formatMessage({ id: 'systemConfig.delete.failed' })}
          type="error"
          onClose={() => remove.reset()}
        />
      ) : null}
      <Space wrap>
        <Button
          icon={<ReloadOutlined />}
          loading={configs.isFetching}
          onClick={() => void configs.refetch()}
        >
          {intl.formatMessage({ id: 'actions.refresh' })}
        </Button>
        <Button icon={<PlusOutlined />} type="primary" onClick={openCreate}>
          {intl.formatMessage({ id: 'systemConfig.create.action' })}
        </Button>
      </Space>
      <Table<SystemConfigSummary>
        columns={columns}
        dataSource={configs.data?.data ?? []}
        loading={configs.isFetching}
        locale={{
          emptyText: <PageEmpty description={intl.formatMessage({ id: 'systemConfig.empty' })} />,
        }}
        pagination={{
          current: params.current,
          pageSize: params.pageSize,
          pageSizeOptions: OPERATIONS_PAGE_SIZES.map(String),
          showSizeChanger: true,
          total: configs.data?.total ?? 0,
          onChange: (current, pageSize) =>
            setParams((previous) => ({
              current,
              pageSize: isOperationsPageSize(pageSize) ? pageSize : previous.pageSize,
            })),
        }}
        rowKey="id"
        scroll={{ x: 820 }}
      />
      <Drawer
        destroyOnHidden
        open={Boolean(viewing)}
        size={screens.md ? 720 : '100%'}
        title={detail.data?.name ?? intl.formatMessage({ id: 'systemConfig.detail.title' })}
        onClose={closeView}
      >
        {detail.isPending ? <PageLoading rows={7} /> : null}
        {detail.isError ? (
          <PageError
            message={getRequestErrorMessage(detail.error)}
            onRetry={() => void detail.refetch()}
            retryLabel={intl.formatMessage({ id: 'actions.retry' })}
          />
        ) : null}
        {viewing && detail.data ? (
          <Space orientation="vertical" size="large" className="w-full">
            <Descriptions
              column={1}
              items={[
                {
                  key: 'format',
                  label: intl.formatMessage({ id: 'systemConfig.field.format' }),
                  children: detail.data.ext.toUpperCase(),
                },
                {
                  key: 'remark',
                  label: intl.formatMessage({ id: 'systemConfig.field.remark' }),
                  children: detail.data.remark || '—',
                },
                {
                  key: 'updatedAt',
                  label: intl.formatMessage({ id: 'systemConfig.field.updatedAt' }),
                  children: formatDate(detail.data.updatedAt),
                },
              ]}
            />
            <Typography.Title level={5}>
              {intl.formatMessage({ id: 'systemConfig.field.content' })}
            </Typography.Title>
            <pre className="max-h-[60vh] overflow-auto whitespace-pre-wrap break-all rounded-lg bg-neutral-950 p-4 text-xs text-neutral-100">
              {detail.data.content || '—'}
            </pre>
          </Space>
        ) : null}
      </Drawer>
      <Modal
        destroyOnHidden
        forceRender
        confirmLoading={save.isPending}
        okButtonProps={{ disabled: editing !== 'create' && !detail.data }}
        open={Boolean(editing)}
        title={intl.formatMessage({
          id: editing === 'create' ? 'systemConfig.create.title' : 'systemConfig.edit.title',
        })}
        width={760}
        onCancel={closeEditor}
        onOk={() => form.submit()}
      >
        {editing !== 'create' && detail.isPending ? <PageLoading rows={7} /> : null}
        {editorError ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(editorError)}
            showIcon
            type="error"
          />
        ) : null}
        {editing === 'create' || detail.data ? (
          <Form<SystemConfigWriteValues>
            form={form}
            layout="vertical"
            onFinish={(values) => save.mutate(values)}
          >
            <Form.Item
              name="name"
              label={intl.formatMessage({ id: 'systemConfig.field.name' })}
              rules={[{ required: true }, { max: 128 }, { pattern: /^[^/\\\0]+$/ }]}
            >
              <Input disabled={editing !== 'create' && editing?.builtIn} autoComplete="off" />
            </Form.Item>
            <Form.Item
              name="ext"
              label={intl.formatMessage({ id: 'systemConfig.field.format' })}
              rules={[{ required: true }]}
            >
              <Select
                disabled={editing !== 'create' && editing?.builtIn}
                options={(['json', 'yaml', 'yml'] as const).map((value) => ({
                  value,
                  label: value.toUpperCase(),
                }))}
              />
            </Form.Item>
            <Form.Item
              name="content"
              label={intl.formatMessage({ id: 'systemConfig.field.content' })}
              extra={intl.formatMessage({ id: 'systemConfig.content.help' })}
              rules={[{ validator: validateContent }]}
            >
              <Input.TextArea
                autoSize={{ minRows: 12, maxRows: 24 }}
                className="font-mono"
                maxLength={MAX_SYSTEM_CONFIG_BYTES}
                spellCheck={false}
              />
            </Form.Item>
            <Form.Item
              name="remark"
              label={intl.formatMessage({ id: 'systemConfig.field.remark' })}
              rules={[{ max: 255 }]}
            >
              <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} />
            </Form.Item>
          </Form>
        ) : null}
      </Modal>
    </Space>
  );
}
