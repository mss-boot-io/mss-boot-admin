import DownloadOutlined from '@ant-design/icons/DownloadOutlined';
import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import SearchOutlined from '@ant-design/icons/SearchOutlined';
import { getRequestErrorMessage, getRequestStatus } from '@mss-admin-core/shared/api/errors';
import {
  PageEmpty,
  PageError,
  PageForbidden,
  PageLoading,
} from '@mss-admin-core/shared/design-system/PageState';
import ResponsiveEntityTable from '@mss-admin-core/shared/design-system/ResponsiveEntityTable';
import type { PagePresentationRuntime } from '@mss-admin-core/shared/presentation/runtime';
import {
  resolveTablePresentation,
  usePresentationPageParams,
  usePresentationSearchExpansion,
} from '@mss-admin-core/shared/presentation/table';
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
  type TableColumnsType,
  Tabs,
  Tag,
  Typography,
} from 'antd';
import type { Dayjs } from 'dayjs';
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
import {
  auditLogPresentationListComponents,
  auditLogPresentationSearchComponents,
  loginLogPresentationListComponents,
  loginLogPresentationSearchComponents,
  runtimeLogPresentationListComponents,
  runtimeLogPresentationSearchComponents,
} from './tablePresentation';

interface LogViewerProps {
  auditPresentationRuntime: PagePresentationRuntime;
  canExportRuntime: boolean;
  canReadRuntime: boolean;
  loginPresentationRuntime: PagePresentationRuntime;
  runtimePresentationRuntime: PagePresentationRuntime;
}

interface LoginLogParams {
  current: number;
  pageSize: OperationsPageSize;
  username?: string;
}

interface AuditLogParams {
  current: number;
  pageSize: OperationsPageSize;
  type: AuditLogType | 'all';
  username?: string;
}

type RuntimeLogTableParams = Omit<RuntimeLogParams, 'page'> & { current: number };

const initialLoginParams: LoginLogParams = { current: 1, pageSize: 20 };
const initialAuditParams: AuditLogParams = { current: 1, pageSize: 20, type: 'all' };
const initialRuntimeParams: RuntimeLogTableParams = { current: 1, pageSize: 20 };

const operationalMessageIDs: Readonly<Record<string, string>> = {
  'authentication failed': 'log.message.authenticationFailed',
  'by-session': 'log.message.forcedLogout',
  'login success': 'log.message.loginSuccess',
  'self-logout': 'log.message.selfLogout',
};

function formatOperationalMessage(value: string | undefined, translate: (id: string) => string) {
  const normalized = value?.trim();
  if (!normalized) return '—';
  const messageID = operationalMessageIDs[normalized];
  return messageID ? translate(messageID) : normalized;
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

function LoginLogTable({ presentationRuntime }: { presentationRuntime: PagePresentationRuntime }) {
  const intl = useIntl();
  const [form] = Form.useForm<{ username?: string }>();
  const presentation = presentationRuntime.model;
  const configuredPageSize = isOperationsPageSize(presentation.list.pageSize)
    ? presentation.list.pageSize
    : initialLoginParams.pageSize;
  const [params, setParams] = usePresentationPageParams(initialLoginParams, configuredPageSize);
  const searchPresentation = usePresentationSearchExpansion(presentation.search.collapsedByDefault);
  const logs = useLoginLogPage(params);
  const status = getRequestStatus(logs.error);
  const compiledColumns: TableColumnsType<LoginLogEntry> = [
    {
      title: intl.formatMessage({ id: 'log.field.username' }),
      dataIndex: 'username',
      width: 160,
      render: (value: string) => value || '—',
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
      render: (value: string) =>
        formatOperationalMessage(value, (id) => intl.formatMessage({ id })),
    },
    {
      title: intl.formatMessage({ id: 'log.field.loginAt' }),
      dataIndex: 'loginAt',
      width: 190,
      render: (value: string) => formatDate(intl.locale, value),
    },
  ];
  const tablePresentation = resolveTablePresentation({
    compiledColumns,
    fallbackPageSize: initialLoginParams.pageSize,
    isPageSize: isOperationsPageSize,
    listComponents: loginLogPresentationListComponents,
    mobileColumnKeys: ['username', 'ip', 'location', 'status', 'message', 'loginAt'],
    model: presentation,
    searchComponents: loginLogPresentationSearchComponents,
  });
  const usernameSearch = tablePresentation.searchFields.get('username');

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
        onFinish={(values: { username?: string }) =>
          setParams((current) => ({
            ...current,
            current: 1,
            username: values.username?.trim() || undefined,
          }))
        }
      >
        {searchPresentation.expanded && usernameSearch ? (
          <Form.Item
            name="username"
            label={usernameSearch.label}
            extra={usernameSearch.help}
            style={{ order: usernameSearch.order }}
          >
            <Input allowClear maxLength={255} placeholder={usernameSearch.placeholder} />
          </Form.Item>
        ) : null}
        <Form.Item style={{ order: 10_000 }}>
          <Space wrap>
            {searchPresentation.expanded ? (
              <>
                <Button htmlType="submit" icon={<SearchOutlined />} type="primary">
                  {intl.formatMessage({ id: 'actions.search' })}
                </Button>
                <Button
                  onClick={() => {
                    form.resetFields();
                    setParams({
                      ...initialLoginParams,
                      pageSize: tablePresentation.pageSize,
                    });
                  }}
                >
                  {intl.formatMessage({ id: 'actions.reset' })}
                </Button>
              </>
            ) : (
              <Button icon={<SearchOutlined />} onClick={searchPresentation.expand}>
                {intl.formatMessage({ id: 'actions.search' })}
              </Button>
            )}
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
      <ResponsiveEntityTable<LoginLogEntry>
        columns={tablePresentation.columns}
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
        mobileColumnKeys={tablePresentation.mobileColumnKeys}
        rowKey="id"
        scroll={{ x: 920 }}
        size={tablePresentation.density === 'compact' ? 'small' : tablePresentation.density}
      />
    </Space>
  );
}

function AuditLogTable({ presentationRuntime }: { presentationRuntime: PagePresentationRuntime }) {
  const intl = useIntl();
  const [form] = Form.useForm<{ type: AuditLogType | 'all'; username?: string }>();
  const presentation = presentationRuntime.model;
  const configuredPageSize = isOperationsPageSize(presentation.list.pageSize)
    ? presentation.list.pageSize
    : initialAuditParams.pageSize;
  const [params, setParams] = usePresentationPageParams(initialAuditParams, configuredPageSize);
  const searchPresentation = usePresentationSearchExpansion(presentation.search.collapsedByDefault);
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
  const compiledColumns: TableColumnsType<AuditLogEntry> = [
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
      title: intl.formatMessage({ id: 'log.field.message' }),
      dataIndex: 'message',
      ellipsis: true,
      responsive: ['xxl'],
      render: (value: string) =>
        formatOperationalMessage(value, (id) => intl.formatMessage({ id })),
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
  const tablePresentation = resolveTablePresentation({
    compiledColumns,
    fallbackPageSize: initialAuditParams.pageSize,
    isPageSize: isOperationsPageSize,
    listComponents: auditLogPresentationListComponents,
    mobileColumnKeys: [
      'username',
      'type',
      'action',
      'resource',
      'message',
      'status',
      'duration',
      'createdAt',
    ],
    model: presentation,
    searchComponents: auditLogPresentationSearchComponents,
  });
  const usernameSearch = tablePresentation.searchFields.get('username');
  const typeSearch = tablePresentation.searchFields.get('type');

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
        onFinish={(values: { type: AuditLogType | 'all'; username?: string }) =>
          setParams((current) => ({
            ...current,
            current: 1,
            username: values.username?.trim() || undefined,
            type: values.type,
          }))
        }
      >
        {searchPresentation.expanded && usernameSearch ? (
          <Form.Item
            name="username"
            label={usernameSearch.label}
            extra={usernameSearch.help}
            style={{ order: usernameSearch.order }}
          >
            <Input allowClear maxLength={255} placeholder={usernameSearch.placeholder} />
          </Form.Item>
        ) : null}
        {searchPresentation.expanded && typeSearch ? (
          <Form.Item
            name="type"
            label={typeSearch.label}
            extra={typeSearch.help}
            style={{ order: typeSearch.order }}
          >
            <Select
              className="min-w-36"
              placeholder={typeSearch.placeholder}
              options={types.map((value) => ({
                value,
                label: intl.formatMessage({ id: `log.type.${value}` }),
              }))}
            />
          </Form.Item>
        ) : null}
        <Form.Item style={{ order: 10_000 }}>
          <Space wrap>
            {searchPresentation.expanded ? (
              <>
                <Button htmlType="submit" icon={<SearchOutlined />} type="primary">
                  {intl.formatMessage({ id: 'actions.search' })}
                </Button>
                <Button
                  onClick={() => {
                    form.resetFields();
                    setParams({
                      ...initialAuditParams,
                      pageSize: tablePresentation.pageSize,
                    });
                  }}
                >
                  {intl.formatMessage({ id: 'actions.reset' })}
                </Button>
              </>
            ) : (
              <Button icon={<SearchOutlined />} onClick={searchPresentation.expand}>
                {intl.formatMessage({ id: 'actions.search' })}
              </Button>
            )}
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
      <ResponsiveEntityTable<AuditLogEntry>
        columns={tablePresentation.columns}
        dataSource={logs.data?.data ?? []}
        expandable={{
          expandedRowRender: (row) => (
            <Typography.Paragraph className="mb-0 whitespace-pre-wrap break-all">
              {formatOperationalMessage(row.message, (id) => intl.formatMessage({ id }))}
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
        mobileColumnKeys={tablePresentation.mobileColumnKeys}
        rowKey="id"
        scroll={{ x: 1_080 }}
        size={tablePresentation.density === 'compact' ? 'small' : tablePresentation.density}
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

function RuntimeLogTable({
  canExport,
  presentationRuntime,
}: {
  canExport: boolean;
  presentationRuntime: PagePresentationRuntime;
}) {
  const intl = useIntl();
  const [form] = Form.useForm<RuntimeFilterValues>();
  const presentation = presentationRuntime.model;
  const configuredPageSize = isOperationsPageSize(presentation.list.pageSize)
    ? presentation.list.pageSize
    : initialRuntimeParams.pageSize;
  const [params, setParams] = usePresentationPageParams(initialRuntimeParams, configuredPageSize);
  const searchPresentation = usePresentationSearchExpansion(presentation.search.collapsedByDefault);
  const requestParams: RuntimeLogParams = {
    page: params.current,
    pageSize: params.pageSize,
    level: params.level,
    keyword: params.keyword,
    startTime: params.startTime,
    endTime: params.endTime,
  };
  const logs = useRuntimeLogPage(requestParams, true);
  const files = useRuntimeLogFiles(true);
  const status = getRequestStatus(logs.error);
  const compiledColumns: TableColumnsType<RuntimeLogRow> = [
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
  const tablePresentation = resolveTablePresentation({
    compiledColumns,
    fallbackPageSize: initialRuntimeParams.pageSize,
    isPageSize: isOperationsPageSize,
    listComponents: runtimeLogPresentationListComponents,
    mobileColumnKeys: ['timestamp', 'level', 'message'],
    model: presentation,
    searchComponents: runtimeLogPresentationSearchComponents,
  });
  const levelSearch = tablePresentation.searchFields.get('level');
  const keywordSearch = tablePresentation.searchFields.get('keyword');
  const timeRangeSearch = tablePresentation.searchFields.get('timeRange');

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
  const exportPath = runtimeLogExportPath(requestParams);
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
            current: 1,
            level: values.level || undefined,
            keyword: values.keyword?.trim() || undefined,
            startTime: values.range?.[0].toISOString(),
            endTime: values.range?.[1].toISOString(),
          }))
        }
      >
        <Row align="bottom" gutter={16}>
          {searchPresentation.expanded && levelSearch ? (
            <Col xs={24} sm={12} lg={5} style={{ order: levelSearch.order }}>
              <Form.Item name="level" label={levelSearch.label} extra={levelSearch.help}>
                <Select
                  placeholder={levelSearch.placeholder}
                  options={['', 'trace', 'debug', 'info', 'warn', 'error', 'fatal'].map(
                    (value) => ({
                      value,
                      label: value
                        ? value.toUpperCase()
                        : intl.formatMessage({ id: 'log.level.all' }),
                    }),
                  )}
                />
              </Form.Item>
            </Col>
          ) : null}
          {searchPresentation.expanded && keywordSearch ? (
            <Col xs={24} sm={12} lg={7} style={{ order: keywordSearch.order }}>
              <Form.Item name="keyword" label={keywordSearch.label} extra={keywordSearch.help}>
                <Input allowClear maxLength={128} placeholder={keywordSearch.placeholder} />
              </Form.Item>
            </Col>
          ) : null}
          {searchPresentation.expanded && timeRangeSearch ? (
            <Col xs={24} lg={12} style={{ order: timeRangeSearch.order }}>
              <Form.Item name="range" label={timeRangeSearch.label} extra={timeRangeSearch.help}>
                <DatePicker.RangePicker
                  className="w-full"
                  placeholder={
                    timeRangeSearch.placeholder
                      ? [timeRangeSearch.placeholder, timeRangeSearch.placeholder]
                      : undefined
                  }
                  showTime
                />
              </Form.Item>
            </Col>
          ) : null}
          <Col xs={24} style={{ order: 10_000 }}>
            <Form.Item>
              <Space wrap>
                {searchPresentation.expanded ? (
                  <>
                    <Button htmlType="submit" icon={<SearchOutlined />} type="primary">
                      {intl.formatMessage({ id: 'actions.search' })}
                    </Button>
                    <Button
                      onClick={() => {
                        form.resetFields();
                        setParams({
                          ...initialRuntimeParams,
                          pageSize: tablePresentation.pageSize,
                        });
                      }}
                    >
                      {intl.formatMessage({ id: 'actions.reset' })}
                    </Button>
                  </>
                ) : (
                  <Button icon={<SearchOutlined />} onClick={searchPresentation.expand}>
                    {intl.formatMessage({ id: 'actions.search' })}
                  </Button>
                )}
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
      <ResponsiveEntityTable<RuntimeLogRow>
        columns={tablePresentation.columns}
        dataSource={(logs.data?.list ?? []).map((entry, index) => ({
          ...entry,
          rowKey: `${params.current}:${params.pageSize}:${index}`,
        }))}
        expandable={{
          expandedRowRender: (row) => (
            <pre className="m-0 whitespace-pre-wrap break-all text-xs">{row.raw}</pre>
          ),
        }}
        loading={logs.isFetching}
        locale={{ emptyText: <PageEmpty description={intl.formatMessage({ id: 'log.empty' })} /> }}
        pagination={{
          current: params.current,
          pageSize: params.pageSize,
          pageSizeOptions: OPERATIONS_PAGE_SIZES.map(String),
          showSizeChanger: true,
          total: logs.data?.total ?? 0,
          onChange: (page, pageSize) =>
            setParams((previous) => ({
              ...previous,
              current: page,
              pageSize: isOperationsPageSize(pageSize) ? pageSize : previous.pageSize,
            })),
        }}
        mobileColumnKeys={tablePresentation.mobileColumnKeys}
        rowKey="rowKey"
        scroll={{ x: 760 }}
        size={tablePresentation.density === 'compact' ? 'small' : tablePresentation.density}
      />
    </Space>
  );
}

export default function LogViewer({
  auditPresentationRuntime,
  canExportRuntime,
  canReadRuntime,
  loginPresentationRuntime,
  runtimePresentationRuntime,
}: LogViewerProps) {
  const items = [
    {
      key: 'login',
      label: loginPresentationRuntime.model.title,
      children: <LoginLogTable presentationRuntime={loginPresentationRuntime} />,
    },
    {
      key: 'audit',
      label: auditPresentationRuntime.model.title,
      children: <AuditLogTable presentationRuntime={auditPresentationRuntime} />,
    },
    ...(canReadRuntime
      ? [
          {
            key: 'runtime',
            label: runtimePresentationRuntime.model.title,
            children: (
              <RuntimeLogTable
                canExport={canExportRuntime}
                presentationRuntime={runtimePresentationRuntime}
              />
            ),
          },
        ]
      : []),
  ];
  return <Tabs destroyOnHidden items={items} />;
}
