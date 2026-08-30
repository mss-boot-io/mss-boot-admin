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
import { queryClient, queryKeys } from '@mss-admin-core/shared/query/client';
import { useMutation } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Col,
  Form,
  Input,
  Popconfirm,
  Row,
  Select,
  Space,
  type TableColumnsType,
  Tag,
  Typography,
} from 'antd';
import { useState } from 'react';
import { sessionAPI } from './api';
import {
  getOnlineSessionStatus,
  isOnlineSessionPageSize,
  ONLINE_SESSION_PAGE_SIZES,
  type OnlineSession,
  type OnlineSessionListParams,
  type OnlineSessionStatusFilter,
} from './contract';
import { sessionDeviceSummary } from './device';
import { useOnlineSessionPage } from './query';
import SessionDetailDrawer from './SessionDetailDrawer';
import {
  onlineSessionPresentationListComponents,
  onlineSessionPresentationMobileFields,
  onlineSessionPresentationSearchComponents,
} from './tablePresentation';

interface SessionFilterValues {
  ip?: string;
  status: OnlineSessionStatusFilter;
  username?: string;
}

type RevokeTarget = { kind: 'session'; id: string } | { kind: 'user'; id: string };

const initialParams: OnlineSessionListParams = {
  current: 1,
  pageSize: 20,
  status: 'active',
};

function cleanFilter(value?: string): string | undefined {
  const normalized = value?.trim();
  return normalized || undefined;
}

function statusColor(status: ReturnType<typeof getOnlineSessionStatus>): string {
  if (status === 'active') return 'green';
  if (status === 'revoked') return 'red';
  return 'default';
}

export default function OnlineSessionsView({
  presentationRuntime,
}: {
  presentationRuntime: PagePresentationRuntime;
}) {
  const intl = useIntl();
  const { message } = App.useApp();
  const [form] = Form.useForm<SessionFilterValues>();
  const presentation = presentationRuntime.model;
  const configuredPageSize = isOnlineSessionPageSize(presentation.list.pageSize)
    ? presentation.list.pageSize
    : initialParams.pageSize;
  const [params, setParams] = usePresentationPageParams(initialParams, configuredPageSize);
  const searchPresentation = usePresentationSearchExpansion(presentation.search.collapsedByDefault);
  const [detailID, setDetailID] = useState<string>();
  const sessions = useOnlineSessionPage(params);
  const revoke = useMutation({
    mutationFn: async (target: RevokeTarget) => {
      if (target.kind === 'session') {
        await sessionAPI.revokeOne(target.id);
        return { target, affected: 1 };
      }
      const result = await sessionAPI.revokeUser(target.id);
      return { target, affected: result.affected };
    },
    onSuccess: async ({ target, affected }) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.onlineSessions });
      void message.success(
        target.kind === 'session'
          ? intl.formatMessage({ id: 'sessions.revoke.success' })
          : intl.formatMessage({ id: 'sessions.revokeUser.success' }, { count: affected }),
      );
    },
  });
  const listStatus = getRequestStatus(sessions.error);
  const revokeStatus = getRequestStatus(revoke.error);

  if (listStatus === 403 || revokeStatus === 403) {
    return <PageForbidden message={intl.formatMessage({ id: 'sessions.forbidden' })} />;
  }

  const formatDate = (value: string) =>
    new Intl.DateTimeFormat(intl.locale || 'zh-CN', {
      dateStyle: 'short',
      timeStyle: 'medium',
    }).format(new Date(value));

  const renderStatus = (row: OnlineSession) => {
    const status = getOnlineSessionStatus(row);
    return (
      <Space size="small" wrap>
        <Tag color={statusColor(status)}>
          {intl.formatMessage({ id: `sessions.status.${status}` })}
        </Tag>
        {row.current ? (
          <Tag color="blue">{intl.formatMessage({ id: 'sessions.current' })}</Tag>
        ) : null}
      </Space>
    );
  };

  const renderActions = (row: OnlineSession) => {
    const active = getOnlineSessionStatus(row) === 'active';
    const sessionPending =
      revoke.isPending && revoke.variables?.kind === 'session' && revoke.variables.id === row.id;
    const userPending =
      revoke.isPending && revoke.variables?.kind === 'user' && revoke.variables.id === row.userID;
    return (
      <Space size="small" wrap>
        <Button type="link" size="small" onClick={() => setDetailID(row.id)}>
          {intl.formatMessage({ id: 'sessions.action.detail' })}
        </Button>
        {active ? (
          <Popconfirm
            description={
              row.current
                ? intl.formatMessage({ id: 'sessions.revoke.current.description' })
                : undefined
            }
            title={intl.formatMessage({ id: 'sessions.revoke.confirm' })}
            onConfirm={() => revoke.mutate({ kind: 'session', id: row.id })}
          >
            <Button
              danger
              disabled={revoke.isPending && !sessionPending}
              loading={sessionPending}
              size="small"
              type="link"
            >
              {intl.formatMessage({ id: 'sessions.action.revoke' })}
            </Button>
          </Popconfirm>
        ) : null}
        {active ? (
          <Popconfirm
            description={intl.formatMessage(
              { id: 'sessions.revokeUser.description' },
              { username: row.username },
            )}
            title={intl.formatMessage({ id: 'sessions.revokeUser.confirm' })}
            onConfirm={() => revoke.mutate({ kind: 'user', id: row.userID })}
          >
            <Button
              danger
              disabled={revoke.isPending && !userPending}
              loading={userPending}
              size="small"
              type="link"
            >
              {intl.formatMessage({ id: 'sessions.action.revokeUser' })}
            </Button>
          </Popconfirm>
        ) : null}
      </Space>
    );
  };

  const compiledColumns: TableColumnsType<OnlineSession> = [
    {
      title: intl.formatMessage({ id: 'sessions.field.username' }),
      dataIndex: 'username',
      render: (_, row) => (
        <Button type="link" className="px-0" onClick={() => setDetailID(row.id)}>
          {row.username}
        </Button>
      ),
    },
    {
      title: intl.formatMessage({ id: 'sessions.field.ip' }),
      dataIndex: 'ip',
      responsive: ['md'],
      render: (_, row) => row.ip || '—',
    },
    {
      title: intl.formatMessage({ id: 'sessions.field.device' }),
      dataIndex: 'userAgent',
      ellipsis: true,
      responsive: ['xl'],
      render: (_, row) => (
        <Typography.Text ellipsis={{ tooltip: row.userAgent }} className="block max-w-64">
          {sessionDeviceSummary(
            row.userAgent,
            intl.formatMessage({ id: 'sessions.device.mobile' }),
            intl.formatMessage({ id: 'sessions.device.unknown' }),
          )}
        </Typography.Text>
      ),
    },
    {
      title: intl.formatMessage({ id: 'sessions.field.lastSeenAt' }),
      dataIndex: 'lastSeenAt',
      responsive: ['sm'],
      render: (_, row) => formatDate(row.lastSeenAt),
    },
    {
      title: intl.formatMessage({ id: 'sessions.field.status' }),
      key: 'status',
      render: (_, row) => renderStatus(row),
    },
    {
      title: intl.formatMessage({ id: 'sessions.field.actions' }),
      key: 'actions',
      width: 230,
      render: (_, row) => renderActions(row),
    },
  ];
  const tablePresentation = resolveTablePresentation({
    compiledColumns,
    fallbackPageSize: initialParams.pageSize,
    isPageSize: isOnlineSessionPageSize,
    listComponents: onlineSessionPresentationListComponents,
    mobileColumnKeys: [...onlineSessionPresentationMobileFields, 'actions'],
    model: presentation,
    protectedColumnKeys: ['actions'],
    searchComponents: onlineSessionPresentationSearchComponents,
  });
  const usernameSearch = tablePresentation.searchFields.get('username');
  const ipSearch = tablePresentation.searchFields.get('ip');
  const statusSearch = tablePresentation.searchFields.get('status');

  if (sessions.isPending && !sessions.data) return <PageLoading rows={9} />;
  if (sessions.isError && (listStatus === 401 || !sessions.data)) {
    return (
      <PageError
        message={getRequestErrorMessage(sessions.error)}
        onRetry={() => void sessions.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
        title={intl.formatMessage({ id: 'states.loadError' })}
      />
    );
  }

  return (
    <Space orientation="vertical" size="middle" className="w-full">
      {sessions.isError ? (
        <Alert
          showIcon
          title={intl.formatMessage({ id: 'sessions.refreshFailed' })}
          type="warning"
        />
      ) : null}
      {revoke.isError ? (
        <Alert
          closable
          description={getRequestErrorMessage(revoke.error)}
          showIcon
          title={intl.formatMessage({ id: 'sessions.revoke.failed' })}
          type="error"
          onClose={() => revoke.reset()}
        />
      ) : null}
      <Form<SessionFilterValues>
        form={form}
        initialValues={{ status: 'active' }}
        layout="vertical"
        onFinish={(values) =>
          setParams((current) => ({
            ...current,
            current: 1,
            status: values.status,
            userID: undefined,
            username: cleanFilter(values.username),
            ip: cleanFilter(values.ip),
          }))
        }
      >
        <Row gutter={16} align="bottom">
          {searchPresentation.expanded && usernameSearch ? (
            <Col xs={24} sm={12} lg={5} style={{ order: usernameSearch.order }}>
              <Form.Item name="username" label={usernameSearch.label} extra={usernameSearch.help}>
                <Input allowClear maxLength={255} placeholder={usernameSearch.placeholder} />
              </Form.Item>
            </Col>
          ) : null}
          {searchPresentation.expanded && ipSearch ? (
            <Col xs={24} sm={12} lg={4} style={{ order: ipSearch.order }}>
              <Form.Item name="ip" label={ipSearch.label} extra={ipSearch.help}>
                <Input allowClear maxLength={64} placeholder={ipSearch.placeholder} />
              </Form.Item>
            </Col>
          ) : null}
          {searchPresentation.expanded && statusSearch ? (
            <Col xs={24} sm={12} lg={4} style={{ order: statusSearch.order }}>
              <Form.Item name="status" label={statusSearch.label} extra={statusSearch.help}>
                <Select
                  placeholder={statusSearch.placeholder}
                  options={(['active', 'revoked', 'expired', 'all'] as const).map((value) => ({
                    value,
                    label: intl.formatMessage({ id: `sessions.status.${value}` }),
                  }))}
                />
              </Form.Item>
            </Col>
          ) : null}
          <Col xs={24} lg={6} style={{ order: 10_000 }}>
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
                        setParams({ ...initialParams, pageSize: tablePresentation.pageSize });
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
                  loading={sessions.isFetching}
                  onClick={() => void sessions.refetch()}
                >
                  {intl.formatMessage({ id: 'actions.refresh' })}
                </Button>
              </Space>
            </Form.Item>
          </Col>
        </Row>
      </Form>
      <ResponsiveEntityTable<OnlineSession>
        columns={tablePresentation.columns}
        dataSource={sessions.data?.data ?? []}
        loading={sessions.isFetching}
        locale={{
          emptyText: <PageEmpty description={intl.formatMessage({ id: 'sessions.empty' })} />,
        }}
        mobileColumnKeys={tablePresentation.mobileColumnKeys}
        pagination={{
          current: params.current,
          pageSize: params.pageSize,
          pageSizeOptions: ONLINE_SESSION_PAGE_SIZES.map(String),
          showSizeChanger: true,
          total: sessions.data?.total ?? 0,
          onChange: (current, pageSize) =>
            setParams((previous) => ({
              ...previous,
              current,
              pageSize: isOnlineSessionPageSize(pageSize) ? pageSize : previous.pageSize,
            })),
        }}
        rowKey="id"
        scroll={{ x: 760 }}
        size={tablePresentation.density === 'compact' ? 'small' : tablePresentation.density}
      />
      <SessionDetailDrawer
        id={detailID}
        open={Boolean(detailID)}
        onClose={() => setDetailID(undefined)}
      />
    </Space>
  );
}
