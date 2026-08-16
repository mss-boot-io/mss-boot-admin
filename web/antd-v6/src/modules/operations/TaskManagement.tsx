import DeleteOutlined from '@ant-design/icons/DeleteOutlined';
import EditOutlined from '@ant-design/icons/EditOutlined';
import PauseCircleOutlined from '@ant-design/icons/PauseCircleOutlined';
import PlayCircleOutlined from '@ant-design/icons/PlayCircleOutlined';
import PlusOutlined from '@ant-design/icons/PlusOutlined';
import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import SearchOutlined from '@ant-design/icons/SearchOutlined';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Col,
  Form,
  Grid,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Table,
  type TableColumnsType,
  Tag,
  Tooltip,
} from 'antd';
import { useEffect, useState } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageEmpty, PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import { queryKeys } from '@/shared/query/client';
import { operationsAPI } from './api';
import {
  isOperationsPageSize,
  OPERATIONS_PAGE_SIZES,
  type OperationsListParams,
  type TaskSummary,
  type TaskWriteValues,
  taskFormValues,
} from './contract';
import { useTask, useTaskFunctions, useTaskPage } from './query';

interface TaskManagementProps {
  root: boolean;
}

interface TaskFilterValues {
  name?: string;
  status: OperationsListParams['status'];
}

const initialParams: OperationsListParams = {
  current: 1,
  pageSize: 20,
  status: 'all',
};

const initialTask: Partial<TaskWriteValues> = {
  provider: 'default',
  protocol: 'https',
  method: 'GET',
  timeout: 30,
  namespace: 'default',
  args: [],
  command: [],
};

function validateJSONObject(value: string | undefined, message: string): Promise<void> {
  if (!value?.trim()) return Promise.resolve();
  try {
    const parsed = JSON.parse(value) as unknown;
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return Promise.reject(new Error(message));
    }
    const entries = Object.entries(parsed as Record<string, unknown>);
    if (
      entries.length > 64 ||
      entries.some(
        ([key, entry]) =>
          !key.trim() ||
          [...key.trim()].length > 128 ||
          typeof entry !== 'string' ||
          [...entry].length > 4_096 ||
          /[\r\n\0]/.test(`${key}${entry}`),
      )
    ) {
      return Promise.reject(new Error(message));
    }
    return Promise.resolve();
  } catch {
    return Promise.reject(new Error(message));
  }
}

export default function TaskManagement({ root }: TaskManagementProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const screens = Grid.useBreakpoint();
  const [filterForm] = Form.useForm<TaskFilterValues>();
  const [editorForm] = Form.useForm<TaskWriteValues>();
  const [params, setParams] = useState<OperationsListParams>(initialParams);
  const [editing, setEditing] = useState<'create' | string>();
  const tasks = useTaskPage(params);
  const detail = useTask(editing && editing !== 'create' ? editing : undefined);
  const provider = Form.useWatch('provider', editorForm) ?? 'default';
  const functions = useTaskFunctions(root && provider === 'func');

  useEffect(() => {
    if (!editing || editing === 'create' || !detail.data) return;
    editorForm.setFieldsValue(taskFormValues(detail.data));
  }, [detail.data, editing, editorForm]);

  const closeEditor = () => {
    setEditing(undefined);
    editorForm.resetFields();
  };

  const save = useMutation({
    mutationFn: (values: TaskWriteValues) => {
      if (!editing) throw new Error('task editor is not open');
      return editing === 'create'
        ? operationsAPI.tasks.create(values)
        : operationsAPI.tasks.update(editing, values);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.tasks });
      closeEditor();
      void message.success(intl.formatMessage({ id: 'task.save.success' }));
    },
  });
  const operate = useMutation({
    mutationFn: ({ id, operation }: { id: string; operation: 'start' | 'stop' }) =>
      operationsAPI.tasks.operate(id, operation),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.tasks });
      void message.success(intl.formatMessage({ id: 'task.operation.success' }));
    },
  });
  const remove = useMutation({
    mutationFn: (id: string) => operationsAPI.tasks.remove(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.tasks });
      void message.success(intl.formatMessage({ id: 'task.delete.success' }));
    },
  });

  const openCreate = () => {
    save.reset();
    editorForm.resetFields();
    editorForm.setFieldsValue(initialTask);
    setEditing('create');
  };
  const openEdit = (task: TaskSummary) => {
    save.reset();
    editorForm.resetFields();
    setEditing(task.id);
  };

  const formatDate = (value?: string) =>
    value
      ? new Intl.DateTimeFormat(intl.locale || 'zh-CN', {
          dateStyle: 'short',
          timeStyle: 'medium',
        }).format(new Date(value))
      : '—';

  const columns: TableColumnsType<TaskSummary> = [
    {
      title: intl.formatMessage({ id: 'task.field.name' }),
      dataIndex: 'name',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'task.field.provider' }),
      dataIndex: 'provider',
      width: 120,
      render: (value: TaskSummary['provider']) => (
        <Tag color={value === 'k8s' ? 'blue' : value === 'func' ? 'purple' : 'default'}>
          {intl.formatMessage({ id: `task.provider.${value}` })}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({ id: 'task.field.schedule' }),
      dataIndex: 'spec',
      width: 180,
      render: (value: string) => <code>{value}</code>,
    },
    {
      title: intl.formatMessage({ id: 'task.field.status' }),
      dataIndex: 'status',
      width: 110,
      render: (value: TaskSummary['status']) => (
        <Tag color={value === 'enabled' ? 'green' : 'default'}>
          {intl.formatMessage({ id: `task.status.${value}` })}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({ id: 'task.field.lastRun' }),
      dataIndex: 'checkedAt',
      responsive: ['lg'],
      width: 190,
      render: (value?: string) => formatDate(value),
    },
    {
      title: intl.formatMessage({ id: 'task.field.remark' }),
      dataIndex: 'remark',
      responsive: ['md'],
      ellipsis: true,
      render: (value: string) => value || '—',
    },
    ...(root
      ? ([
          {
            title: intl.formatMessage({ id: 'task.field.actions' }),
            key: 'actions',
            width: 300,
            fixed: screens.md ? 'right' : undefined,
            render: (_: unknown, task: TaskSummary) => {
              const operation = task.status === 'enabled' ? 'stop' : 'start';
              const pendingOperation = operate.isPending && operate.variables?.id === task.id;
              return (
                <Space size="small" wrap>
                  <Button
                    icon={operation === 'start' ? <PlayCircleOutlined /> : <PauseCircleOutlined />}
                    loading={pendingOperation}
                    size="small"
                    type="link"
                    onClick={() => operate.mutate({ id: task.id, operation })}
                  >
                    {intl.formatMessage({ id: `task.operation.${operation}` })}
                  </Button>
                  <Button
                    icon={<EditOutlined />}
                    size="small"
                    type="link"
                    onClick={() => openEdit(task)}
                  >
                    {intl.formatMessage({ id: 'actions.edit' })}
                  </Button>
                  <Tooltip
                    title={
                      task.status === 'enabled'
                        ? intl.formatMessage({ id: 'task.delete.stopFirst' })
                        : undefined
                    }
                  >
                    <span>
                      <Popconfirm
                        description={intl.formatMessage({ id: 'task.delete.description' })}
                        disabled={task.status === 'enabled'}
                        title={intl.formatMessage({ id: 'task.delete.confirm' })}
                        onConfirm={() => remove.mutate(task.id)}
                      >
                        <Button
                          danger
                          disabled={task.status === 'enabled'}
                          icon={<DeleteOutlined />}
                          loading={remove.isPending && remove.variables === task.id}
                          size="small"
                          type="link"
                        >
                          {intl.formatMessage({ id: 'actions.delete' })}
                        </Button>
                      </Popconfirm>
                    </span>
                  </Tooltip>
                </Space>
              );
            },
          },
        ] as TableColumnsType<TaskSummary>)
      : []),
  ];

  const listStatus = getRequestStatus(tasks.error);
  if (listStatus === 403) {
    return <PageForbidden message={intl.formatMessage({ id: 'task.forbidden.read' })} />;
  }
  if (tasks.isPending && !tasks.data) return <PageLoading rows={8} />;
  if (tasks.isError && !tasks.data) {
    return (
      <PageError
        message={getRequestErrorMessage(tasks.error)}
        onRetry={() => void tasks.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
        title={intl.formatMessage({ id: 'states.loadError' })}
      />
    );
  }

  const editorError = save.error ?? detail.error;
  return (
    <Space orientation="vertical" size="middle" className="w-full">
      {!root ? (
        <Alert showIcon title={intl.formatMessage({ id: 'task.readOnly' })} type="info" />
      ) : null}
      {tasks.isError ? (
        <Alert
          showIcon
          title={intl.formatMessage({ id: 'operations.refreshFailed' })}
          type="warning"
        />
      ) : null}
      {operate.isError || remove.isError ? (
        <Alert
          closable
          description={getRequestErrorMessage(operate.error ?? remove.error)}
          showIcon
          title={intl.formatMessage({ id: 'task.operation.failed' })}
          type="error"
          onClose={() => {
            operate.reset();
            remove.reset();
          }}
        />
      ) : null}
      <Form<TaskFilterValues>
        form={filterForm}
        initialValues={{ status: 'all' }}
        layout="vertical"
        onFinish={(values) =>
          setParams((current) => ({
            ...current,
            current: 1,
            name: values.name?.trim() || undefined,
            status: values.status,
          }))
        }
      >
        <Row align="bottom" gutter={16}>
          <Col xs={24} sm={12} lg={8}>
            <Form.Item name="name" label={intl.formatMessage({ id: 'task.field.name' })}>
              <Input allowClear maxLength={255} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} lg={6}>
            <Form.Item name="status" label={intl.formatMessage({ id: 'task.field.status' })}>
              <Select
                options={(['all', 'enabled', 'disabled'] as const).map((value) => ({
                  value,
                  label: intl.formatMessage({ id: `task.status.${value}` }),
                }))}
              />
            </Form.Item>
          </Col>
          <Col xs={24} lg={10}>
            <Form.Item>
              <Space wrap>
                <Button htmlType="submit" icon={<SearchOutlined />} type="primary">
                  {intl.formatMessage({ id: 'actions.search' })}
                </Button>
                <Button
                  onClick={() => {
                    filterForm.resetFields();
                    setParams(initialParams);
                  }}
                >
                  {intl.formatMessage({ id: 'actions.reset' })}
                </Button>
                <Button
                  icon={<ReloadOutlined />}
                  loading={tasks.isFetching}
                  onClick={() => void tasks.refetch()}
                >
                  {intl.formatMessage({ id: 'actions.refresh' })}
                </Button>
                {root ? (
                  <Button icon={<PlusOutlined />} type="primary" onClick={openCreate}>
                    {intl.formatMessage({ id: 'task.create.action' })}
                  </Button>
                ) : null}
              </Space>
            </Form.Item>
          </Col>
        </Row>
      </Form>
      <Table<TaskSummary>
        columns={columns}
        dataSource={tasks.data?.data ?? []}
        loading={tasks.isFetching}
        locale={{ emptyText: <PageEmpty description={intl.formatMessage({ id: 'task.empty' })} /> }}
        pagination={{
          current: params.current,
          pageSize: params.pageSize,
          pageSizeOptions: OPERATIONS_PAGE_SIZES.map(String),
          showSizeChanger: true,
          total: tasks.data?.total ?? 0,
          onChange: (current, pageSize) =>
            setParams((previous) => ({
              ...previous,
              current,
              pageSize: isOperationsPageSize(pageSize) ? pageSize : previous.pageSize,
            })),
        }}
        rowKey="id"
        scroll={{ x: root ? 1_080 : 760 }}
      />
      <Modal
        destroyOnHidden
        confirmLoading={save.isPending}
        okButtonProps={{ disabled: editing !== 'create' && !detail.data }}
        open={Boolean(editing)}
        title={intl.formatMessage({
          id: editing === 'create' ? 'task.create.title' : 'task.edit.title',
        })}
        width={760}
        onCancel={closeEditor}
        onOk={() => editorForm.submit()}
      >
        {editing !== 'create' && detail.isPending ? <PageLoading rows={6} /> : null}
        {editorError ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(editorError)}
            showIcon
            type="error"
          />
        ) : null}
        {editing === 'create' || detail.data ? (
          <Form<TaskWriteValues>
            form={editorForm}
            initialValues={initialTask}
            layout="vertical"
            onFinish={(values) => save.mutate(values)}
          >
            <Row gutter={16}>
              <Col xs={24} md={14}>
                <Form.Item
                  name="name"
                  label={intl.formatMessage({ id: 'task.field.name' })}
                  rules={[{ required: true }, { max: 255 }]}
                >
                  <Input autoComplete="off" />
                </Form.Item>
              </Col>
              <Col xs={24} md={10}>
                <Form.Item
                  name="provider"
                  label={intl.formatMessage({ id: 'task.field.provider' })}
                  rules={[{ required: true }]}
                >
                  <Select
                    disabled={editing !== 'create'}
                    options={(['default', 'func', 'k8s'] as const).map((value) => ({
                      value,
                      label: intl.formatMessage({ id: `task.provider.${value}` }),
                    }))}
                  />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} md={16}>
                <Form.Item
                  name="spec"
                  label={intl.formatMessage({ id: 'task.field.schedule' })}
                  extra={intl.formatMessage({
                    id: provider === 'k8s' ? 'task.schedule.k8sHelp' : 'task.schedule.help',
                  })}
                  rules={[{ required: true }, { max: 255 }]}
                >
                  <Input
                    autoComplete="off"
                    placeholder={provider === 'k8s' ? '*/5 * * * *' : '0 */5 * * * *'}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item
                  name="timeout"
                  label={intl.formatMessage({ id: 'task.field.timeout' })}
                  rules={[{ required: true, type: 'number', min: 1, max: 3600 }]}
                >
                  <InputNumber className="w-full" min={1} max={3600} />
                </Form.Item>
              </Col>
            </Row>
            {provider === 'default' ? (
              <>
                <Row gutter={16}>
                  <Col xs={24} md={7}>
                    <Form.Item
                      name="protocol"
                      label={intl.formatMessage({ id: 'task.field.protocol' })}
                      rules={[{ required: true }]}
                    >
                      <Select
                        options={['http', 'https'].map((value) => ({ value, label: value }))}
                      />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={17}>
                    <Form.Item
                      name="endpoint"
                      label={intl.formatMessage({ id: 'task.field.endpoint' })}
                      extra={intl.formatMessage({ id: 'task.endpoint.help' })}
                      rules={[{ required: true }, { max: 255 }]}
                    >
                      <Input autoComplete="off" placeholder="service.example.com/health" />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item
                  name="method"
                  label={intl.formatMessage({ id: 'task.field.method' })}
                  rules={[{ required: true }]}
                >
                  <Select
                    options={['GET', 'POST', 'PUT', 'DELETE'].map((value) => ({
                      value,
                      label: value,
                    }))}
                  />
                </Form.Item>
                <Form.Item name="body" label={intl.formatMessage({ id: 'task.field.body' })}>
                  <Input.TextArea autoSize={{ minRows: 3, maxRows: 10 }} maxLength={64 * 1024} />
                </Form.Item>
                <Form.Item
                  name="metadata"
                  label={intl.formatMessage({ id: 'task.field.metadata' })}
                  rules={[
                    {
                      validator: (_, value) =>
                        validateJSONObject(
                          value,
                          intl.formatMessage({ id: 'task.metadata.invalid' }),
                        ),
                    },
                  ]}
                >
                  <Input.TextArea autoSize={{ minRows: 2, maxRows: 8 }} maxLength={16 * 1024} />
                </Form.Item>
              </>
            ) : null}
            {provider === 'func' ? (
              <>
                <Form.Item
                  name="method"
                  label={intl.formatMessage({ id: 'task.field.function' })}
                  rules={[{ required: true }]}
                >
                  <Select
                    loading={functions.isFetching}
                    options={(functions.data ?? []).map((value) => ({ value, label: value }))}
                    showSearch
                  />
                </Form.Item>
                <Form.Item name="args" label={intl.formatMessage({ id: 'task.field.args' })}>
                  <Select mode="tags" maxCount={32} tokenSeparators={[',']} />
                </Form.Item>
              </>
            ) : null}
            {provider === 'k8s' ? (
              <>
                <Row gutter={16}>
                  <Col xs={24} md={12}>
                    <Form.Item
                      name="cluster"
                      label={intl.formatMessage({ id: 'task.field.cluster' })}
                      rules={[{ required: true }, { max: 50 }]}
                    >
                      <Input disabled={editing !== 'create'} autoComplete="off" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item
                      name="namespace"
                      label={intl.formatMessage({ id: 'task.field.namespace' })}
                      rules={[{ required: true }, { max: 63 }]}
                    >
                      <Input disabled={editing !== 'create'} autoComplete="off" />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item
                  name="image"
                  label={intl.formatMessage({ id: 'task.field.image' })}
                  rules={[{ required: true }, { max: 255 }]}
                >
                  <Input autoComplete="off" />
                </Form.Item>
                <Form.Item name="command" label={intl.formatMessage({ id: 'task.field.command' })}>
                  <Select mode="tags" maxCount={32} tokenSeparators={[',']} />
                </Form.Item>
                <Form.Item name="args" label={intl.formatMessage({ id: 'task.field.args' })}>
                  <Select mode="tags" maxCount={32} tokenSeparators={[',']} />
                </Form.Item>
              </>
            ) : null}
            <Form.Item
              name="remark"
              label={intl.formatMessage({ id: 'task.field.remark' })}
              rules={[{ max: 4096 }]}
            >
              <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} />
            </Form.Item>
          </Form>
        ) : null}
      </Modal>
    </Space>
  );
}
