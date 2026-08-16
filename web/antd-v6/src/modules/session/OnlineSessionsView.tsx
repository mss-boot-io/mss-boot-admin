import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import SearchOutlined from '@ant-design/icons/SearchOutlined';
import { useMutation } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Grid,
  Input,
  List,
  Popconfirm,
  Row,
  Select,
  Space,
  Table,
  type TableColumnsType,
  Tag,
  Typography,
} from 'antd';
import { useState } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageEmpty, PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import { queryClient, queryKeys } from '@/shared/query/client';
import { sessionAPI } from './api';
import {
  getOnlineSessionStatus,
  isOnlineSessionPageSize,
  ONLINE_SESSION_PAGE_SIZES,
  type OnlineSession,
  type OnlineSessionListParams,
  type OnlineSessionStatusFilter,
} from './contract';
import { useOnlineSessionPage } from './query';
import SessionDetailDrawer from './SessionDetailDrawer';

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

export default function OnlineSessionsView() {
  const intl = useIntl();
  const { message } = App.useApp();
  const [form] = Form.useForm<SessionFilterValues>();
  const screens = Grid.useBreakpoint();
  const [params, setParams] = useState<OnlineSessionListParams>(initialParams);
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
      <Tag color={statusColor(status)}>
        {intl.formatMessage({ id: `sessions.status.${status}` })}
      </Tag>
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

  const columns: TableColumnsType<OnlineSession> = [
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
      title: intl.formatMessage({ id: 'sessions.field.userAgent' }),
      dataIndex: 'userAgent',
      ellipsis: true,
      responsive: ['xl'],
      render: (_, row) => (
        <Typography.Text ellipsis={{ tooltip: row.userAgent }} className="block max-w-64">
          {row.userAgent || '—'}
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
          <Col xs={24} sm={12} lg={5}>
            <Form.Item
              name="username"
              label={intl.formatMessage({ id: 'sessions.field.username' })}
            >
              <Input allowClear maxLength={255} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} lg={4}>
            <Form.Item name="ip" label={intl.formatMessage({ id: 'sessions.field.ip' })}>
              <Input allowClear maxLength={64} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} lg={4}>
            <Form.Item name="status" label={intl.formatMessage({ id: 'sessions.field.status' })}>
              <Select
                options={(['active', 'revoked', 'expired', 'all'] as const).map((value) => ({
                  value,
                  label: intl.formatMessage({ id: `sessions.status.${value}` }),
                }))}
              />
            </Form.Item>
          </Col>
          <Col xs={24} lg={6}>
            <Form.Item>
              <Space wrap>
                <Button htmlType="submit" icon={<SearchOutlined />} type="primary">
                  {intl.formatMessage({ id: 'actions.search' })}
                </Button>
                <Button
                  onClick={() => {
                    form.resetFields();
                    setParams(initialParams);
                  }}
                >
                  {intl.formatMessage({ id: 'actions.reset' })}
                </Button>
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
      {screens.md === false ? (
        <List<OnlineSession>
          dataSource={sessions.data?.data ?? []}
          loading={sessions.isFetching}
          locale={{
            emptyText: <PageEmpty description={intl.formatMessage({ id: 'sessions.empty' })} />,
          }}
          pagination={{
            current: params.current,
            hideOnSinglePage: true,
            pageSize: params.pageSize,
            simple: true,
            total: sessions.data?.total ?? 0,
            onChange: (current) => setParams((previous) => ({ ...previous, current })),
          }}
          rowKey={(session) => session.id}
          renderItem={(session) => (
            <List.Item style={{ paddingInline: 0 }}>
              <Card
                className="w-full"
                size="small"
                title={
                  <Button type="link" className="px-0" onClick={() => setDetailID(session.id)}>
                    {session.username}
                  </Button>
                }
              >
                <Descriptions
                  column={1}
                  size="small"
                  items={[
                    {
                      key: 'ip',
                      label: intl.formatMessage({ id: 'sessions.field.ip' }),
                      children: session.ip || '—',
                    },
                    {
                      key: 'lastSeenAt',
                      label: intl.formatMessage({ id: 'sessions.field.lastSeenAt' }),
                      children: formatDate(session.lastSeenAt),
                    },
                    {
                      key: 'status',
                      label: intl.formatMessage({ id: 'sessions.field.status' }),
                      children: renderStatus(session),
                    },
                  ]}
                />
                <div className="mt-3">{renderActions(session)}</div>
              </Card>
            </List.Item>
          )}
        />
      ) : (
        <Table<OnlineSession>
          columns={columns}
          dataSource={sessions.data?.data ?? []}
          loading={sessions.isFetching}
          locale={{
            emptyText: <PageEmpty description={intl.formatMessage({ id: 'sessions.empty' })} />,
          }}
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
        />
      )}
      <SessionDetailDrawer
        id={detailID}
        open={Boolean(detailID)}
        onClose={() => setDetailID(undefined)}
      />
    </Space>
  );
}
