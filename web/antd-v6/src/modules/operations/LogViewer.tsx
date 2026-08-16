import DownloadOutlined from '@ant-design/icons/DownloadOutlined';
import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import SearchOutlined from '@ant-design/icons/SearchOutlined';
import { useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Col,
  DatePicker,
  Form,
  Input,
  Row,
  Select,
  Space,
  Table,
  type TableColumnsType,
  Tabs,
  Tag,
  Typography,
} from 'antd';
import type { Dayjs } from 'dayjs';
import { useState } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageEmpty, PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import { runtimeLogExportPath } from './api';
import {
  type AuditLogEntry,
  type AuditLogType,
  isOperationsPageSize,
  type LoginLogEntry,
  OPERATIONS_PAGE_SIZES,
  type OperationalStatus,
  type OperationsPageSize,
  type RuntimeLogEntry,
  type RuntimeLogLevel,
  type RuntimeLogParams,
} from './contract';
import { useAuditLogPage, useLoginLogPage, useRuntimeLogFiles, useRuntimeLogPage } from './query';

interface LogViewerProps {
  canExportRuntime: boolean;
  canReadRuntime: boolean;
}

function formatDate(locale: string, value?: string): string {
  return value
    ? new Intl.DateTimeFormat(locale || 'zh-CN', {
        dateStyle: 'short',
        timeStyle: 'medium',
      }).format(new Date(value))
    : '—';
}

function StatusTag({ status }: { status: OperationalStatus }) {
  const intl = useIntl();
  return (
    <Tag color={status === 'enabled' ? 'green' : status === 'locked' ? 'gold' : 'red'}>
      {intl.formatMessage({ id: status ? `log.status.${status}` : 'log.status.unknown' })}
    </Tag>
  );
}

function LoginLogTable() {
  const intl = useIntl();
  const [form] = Form.useForm<{ userID?: string }>();
  const [params, setParams] = useState<{
    current: number;
    pageSize: OperationsPageSize;
    userID?: string;
  }>({ current: 1, pageSize: 20 });
  const logs = useLoginLogPage(params);
  const status = getRequestStatus(logs.error);
  const columns: TableColumnsType<LoginLogEntry> = [
    {
      title: intl.formatMessage({ id: 'log.field.username' }),
      dataIndex: 'username',
      width: 160,
      render: (value: string) => value || '—',
    },
    {
      title: intl.formatMessage({ id: 'log.field.userID' }),
      dataIndex: 'userID',
      responsive: ['lg'],
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'log.field.ip' }),
      dataIndex: 'ip',
      width: 150,
    },
    {
      title: intl.formatMessage({ id: 'log.field.location' }),
      dataIndex: 'location',
      responsive: ['md'],
      ellipsis: true,
      render: (value: string) => value || '—',
    },
    {
      title: intl.formatMessage({ id: 'log.field.status' }),
      dataIndex: 'status',
      width: 110,
      render: (value: OperationalStatus) => <StatusTag status={value} />,
    },
    {
      title: intl.formatMessage({ id: 'log.field.message' }),
      dataIndex: 'message',
      responsive: ['lg'],
      ellipsis: true,
      render: (value: string) => value || '—',
    },
    {
      title: intl.formatMessage({ id: 'log.field.loginAt' }),
      dataIndex: 'loginAt',
      width: 190,
      render: (value: string) => formatDate(intl.locale, value),
    },
  ];

  if (status === 403) return <PageForbidden />;
  if (logs.isPending && !logs.data) return <PageLoading rows={8} />;
  if (logs.isError && !logs.data) {
    return (
      <PageError
        message={getRequestErrorMessage(logs.error)}
        onRetry={() => void logs.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }
  return (
    <Space orientation="vertical" size="middle" className="w-full">
      {logs.isError ? (
        <Alert
          showIcon
          title={intl.formatMessage({ id: 'operations.refreshFailed' })}
          type="warning"
        />
      ) : null}
      <Form
        form={form}
        layout="inline"
        onFinish={(values: { userID?: string }) =>
          setParams((current) => ({
            ...current,
            current: 1,
            userID: values.userID?.trim() || undefined,
          }))
        }
      >
        <Form.Item name="userID" label={intl.formatMessage({ id: 'log.field.userID' })}>
          <Input allowClear maxLength={64} />
        </Form.Item>
        <Form.Item>
          <Space wrap>
            <Button htmlType="submit" icon={<SearchOutlined />} type="primary">
              {intl.formatMessage({ id: 'actions.search' })}
            </Button>
            <Button
              onClick={() => {
                form.resetFields();
                setParams({ current: 1, pageSize: 20 });
              }}
            >
              {intl.formatMessage({ id: 'actions.reset' })}
            </Button>
            <Button
              icon={<ReloadOutlined />}
              loading={logs.isFetching}
              onClick={() => void logs.refetch()}
            >
              {intl.formatMessage({ id: 'actions.refresh' })}
            </Button>
          </Space>
        </Form.Item>
      </Form>
      <Table<LoginLogEntry>
        columns={columns}
        dataSource={logs.data?.data ?? []}
        loading={logs.isFetching}
        locale={{ emptyText: <PageEmpty description={intl.formatMessage({ id: 'log.empty' })} /> }}
        pagination={{
          current: params.current,
          pageSize: params.pageSize,
          pageSizeOptions: OPERATIONS_PAGE_SIZES.map(String),
          showSizeChanger: true,
          total: logs.data?.total ?? 0,
          onChange: (current, pageSize) =>
            setParams((previous) => ({
              ...previous,
              current,
              pageSize: isOperationsPageSize(pageSize) ? pageSize : previous.pageSize,
            })),
        }}
        rowKey="id"
        scroll={{ x: 920 }}
      />
    </Space>
  );
}

function AuditLogTable() {
  const intl = useIntl();
  const [form] = Form.useForm<{ type: AuditLogType | 'all'; userID?: string }>();
  const [params, setParams] = useState<{
    current: number;
    pageSize: OperationsPageSize;
    type: AuditLogType | 'all';
    userID?: string;
  }>({ current: 1, pageSize: 20, type: 'all' });
  const logs = useAuditLogPage(params);
  const status = getRequestStatus(logs.error);
  const types: readonly (AuditLogType | 'all')[] = [
    'all',
    'login',
    'logout',
    'create',
    'update',
    'delete',
    'export',
    'import',
    'config',
    'security',
  ];
  const columns: TableColumnsType<AuditLogEntry> = [
    {
      title: intl.formatMessage({ id: 'log.field.username' }),
      dataIndex: 'username',
      width: 150,
      render: (value: string) => value || '—',
    },
    {
      title: intl.formatMessage({ id: 'log.field.type' }),
      dataIndex: 'type',
      width: 120,
      render: (value: AuditLogType) => <Tag>{intl.formatMessage({ id: `log.type.${value}` })}</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'log.field.action' }),
      dataIndex: 'action',
      width: 150,
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'log.field.resource' }),
      dataIndex: 'resource',
      responsive: ['md'],
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'log.field.path' }),
      dataIndex: 'path',
      responsive: ['xl'],
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'log.field.status' }),
      dataIndex: 'status',
      width: 110,
      render: (value: OperationalStatus) => <StatusTag status={value} />,
    },
    {
      title: intl.formatMessage({ id: 'log.field.duration' }),
      dataIndex: 'duration',
      responsive: ['lg'],
      width: 110,
      render: (value: number) => `${value} ms`,
    },
    {
      title: intl.formatMessage({ id: 'log.field.createdAt' }),
      dataIndex: 'createdAt',
      width: 190,
      render: (value: string) => formatDate(intl.locale, value),
    },
  ];

  if (status === 403) return <PageForbidden />;
  if (logs.isPending && !logs.data) return <PageLoading rows={8} />;
  if (logs.isError && !logs.data) {
    return (
      <PageError
        message={getRequestErrorMessage(logs.error)}
        onRetry={() => void logs.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }
  return (
    <Space orientation="vertical" size="middle" className="w-full">
      {logs.isError ? (
        <Alert
          showIcon
          title={intl.formatMessage({ id: 'operations.refreshFailed' })}
          type="warning"
        />
      ) : null}
      <Form
        form={form}
        initialValues={{ type: 'all' }}
        layout="inline"
        onFinish={(values: { type: AuditLogType | 'all'; userID?: string }) =>
          setParams((current) => ({
            ...current,
            current: 1,
            userID: values.userID?.trim() || undefined,
            type: values.type,
          }))
        }
      >
        <Form.Item name="userID" label={intl.formatMessage({ id: 'log.field.userID' })}>
          <Input allowClear maxLength={64} />
        </Form.Item>
        <Form.Item name="type" label={intl.formatMessage({ id: 'log.field.type' })}>
          <Select
            className="min-w-36"
            options={types.map((value) => ({
              value,
              label: intl.formatMessage({ id: `log.type.${value}` }),
            }))}
          />
        </Form.Item>
        <Form.Item>
          <Space wrap>
            <Button htmlType="submit" icon={<SearchOutlined />} type="primary">
              {intl.formatMessage({ id: 'actions.search' })}
            </Button>
            <Button
              onClick={() => {
                form.resetFields();
                setParams({ current: 1, pageSize: 20, type: 'all' });
              }}
            >
              {intl.formatMessage({ id: 'actions.reset' })}
            </Button>
            <Button
              icon={<ReloadOutlined />}
              loading={logs.isFetching}
              onClick={() => void logs.refetch()}
            >
              {intl.formatMessage({ id: 'actions.refresh' })}
            </Button>
          </Space>
        </Form.Item>
      </Form>
      <Table<AuditLogEntry>
        columns={columns}
        dataSource={logs.data?.data ?? []}
        expandable={{
          expandedRowRender: (row) => (
            <Typography.Paragraph className="mb-0 whitespace-pre-wrap break-all">
              {row.message || '—'}
            </Typography.Paragraph>
          ),
          rowExpandable: (row) => Boolean(row.message),
        }}
        loading={logs.isFetching}
        locale={{ emptyText: <PageEmpty description={intl.formatMessage({ id: 'log.empty' })} /> }}
        pagination={{
          current: params.current,
          pageSize: params.pageSize,
          pageSizeOptions: OPERATIONS_PAGE_SIZES.map(String),
          showSizeChanger: true,
          total: logs.data?.total ?? 0,
          onChange: (current, pageSize) =>
            setParams((previous) => ({
              ...previous,
              current,
              pageSize: isOperationsPageSize(pageSize) ? pageSize : previous.pageSize,
            })),
        }}
        rowKey="id"
        scroll={{ x: 1_080 }}
      />
    </Space>
  );
}

interface RuntimeFilterValues {
  keyword?: string;
  level: RuntimeLogLevel;
  range?: [Dayjs, Dayjs];
}

interface RuntimeLogRow extends RuntimeLogEntry {
  rowKey: string;
}

function RuntimeLogTable({ canExport }: { canExport: boolean }) {
  const intl = useIntl();
  const [form] = Form.useForm<RuntimeFilterValues>();
  const [params, setParams] = useState<RuntimeLogParams>({ page: 1, pageSize: 20 });
  const logs = useRuntimeLogPage(params, true);
  const files = useRuntimeLogFiles(true);
  const status = getRequestStatus(logs.error);
  const columns: TableColumnsType<RuntimeLogRow> = [
    {
      title: intl.formatMessage({ id: 'log.field.timestamp' }),
      dataIndex: 'timestamp',
      width: 210,
      render: (value: string) => value || '—',
    },
    {
      title: intl.formatMessage({ id: 'log.field.level' }),
      dataIndex: 'level',
      width: 110,
      render: (value: string) => (
        <Tag
          color={
            value === 'error' || value === 'fatal' ? 'red' : value === 'warn' ? 'gold' : 'blue'
          }
        >
          {value ? value.toUpperCase() : 'TEXT'}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({ id: 'log.field.message' }),
      dataIndex: 'message',
      ellipsis: true,
    },
  ];

  if (status === 403) return <PageForbidden />;
  if (logs.isPending && !logs.data) return <PageLoading rows={8} />;
  if (logs.isError && !logs.data) {
    return (
      <PageError
        message={getRequestErrorMessage(logs.error)}
        onRetry={() => void logs.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }
  const exportPath = runtimeLogExportPath(params);
  return (
    <Space orientation="vertical" size="middle" className="w-full">
      {logs.data?.truncated || files.data?.truncated ? (
        <Alert
          description={intl.formatMessage({ id: 'log.runtime.truncated.description' })}
          showIcon
          title={intl.formatMessage({ id: 'log.runtime.truncated.title' })}
          type="warning"
        />
      ) : null}
      {logs.isError || files.isError ? (
        <Alert
          showIcon
          title={intl.formatMessage({ id: 'operations.refreshFailed' })}
          type="warning"
        />
      ) : null}
      <Form<RuntimeFilterValues>
        form={form}
        initialValues={{ level: '' }}
        layout="vertical"
        onFinish={(values) =>
          setParams((current) => ({
            ...current,
            page: 1,
            level: values.level || undefined,
            keyword: values.keyword?.trim() || undefined,
            startTime: values.range?.[0].toISOString(),
            endTime: values.range?.[1].toISOString(),
          }))
        }
      >
        <Row align="bottom" gutter={16}>
          <Col xs={24} sm={12} lg={5}>
            <Form.Item name="level" label={intl.formatMessage({ id: 'log.field.level' })}>
              <Select
                options={['', 'trace', 'debug', 'info', 'warn', 'error', 'fatal'].map((value) => ({
                  value,
                  label: value ? value.toUpperCase() : intl.formatMessage({ id: 'log.level.all' }),
                }))}
              />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} lg={7}>
            <Form.Item name="keyword" label={intl.formatMessage({ id: 'log.field.keyword' })}>
              <Input allowClear maxLength={128} />
            </Form.Item>
          </Col>
          <Col xs={24} lg={12}>
            <Form.Item name="range" label={intl.formatMessage({ id: 'log.field.timeRange' })}>
              <DatePicker.RangePicker className="w-full" showTime />
            </Form.Item>
          </Col>
          <Col xs={24}>
            <Form.Item>
              <Space wrap>
                <Button htmlType="submit" icon={<SearchOutlined />} type="primary">
                  {intl.formatMessage({ id: 'actions.search' })}
                </Button>
                <Button
                  onClick={() => {
                    form.resetFields();
                    setParams({ page: 1, pageSize: 20 });
                  }}
                >
                  {intl.formatMessage({ id: 'actions.reset' })}
                </Button>
                <Button
                  icon={<ReloadOutlined />}
                  loading={logs.isFetching}
                  onClick={() => void logs.refetch()}
                >
                  {intl.formatMessage({ id: 'actions.refresh' })}
                </Button>
                {canExport ? (
                  <Button href={exportPath} icon={<DownloadOutlined />} type="primary">
                    {intl.formatMessage({ id: 'log.export' })}
                  </Button>
                ) : null}
              </Space>
            </Form.Item>
          </Col>
        </Row>
      </Form>
      {files.data?.files.length ? (
        <Typography.Text type="secondary">
          {intl.formatMessage({ id: 'log.runtime.files' }, { files: files.data.files.join(', ') })}
        </Typography.Text>
      ) : null}
      <Table<RuntimeLogRow>
        columns={columns}
        dataSource={(logs.data?.list ?? []).map((entry, index) => ({
          ...entry,
          rowKey: `${params.page}:${params.pageSize}:${index}`,
        }))}
        expandable={{
          expandedRowRender: (row) => (
            <pre className="m-0 whitespace-pre-wrap break-all text-xs">{row.raw}</pre>
          ),
        }}
        loading={logs.isFetching}
        locale={{ emptyText: <PageEmpty description={intl.formatMessage({ id: 'log.empty' })} /> }}
        pagination={{
          current: params.page,
          pageSize: params.pageSize,
          pageSizeOptions: OPERATIONS_PAGE_SIZES.map(String),
          showSizeChanger: true,
          total: logs.data?.total ?? 0,
          onChange: (page, pageSize) =>
            setParams((previous) => ({
              ...previous,
              page,
              pageSize: isOperationsPageSize(pageSize) ? pageSize : previous.pageSize,
            })),
        }}
        rowKey="rowKey"
        scroll={{ x: 760 }}
      />
    </Space>
  );
}

export default function LogViewer({ canExportRuntime, canReadRuntime }: LogViewerProps) {
  const intl = useIntl();
  const items = [
    {
      key: 'login',
      label: intl.formatMessage({ id: 'log.tab.login' }),
      children: <LoginLogTable />,
    },
    {
      key: 'audit',
      label: intl.formatMessage({ id: 'log.tab.audit' }),
      children: <AuditLogTable />,
    },
    ...(canReadRuntime
      ? [
          {
            key: 'runtime',
            label: intl.formatMessage({ id: 'log.tab.runtime' }),
            children: <RuntimeLogTable canExport={canExportRuntime} />,
          },
        ]
      : []),
  ];
  return <Tabs destroyOnHidden items={items} />;
}
