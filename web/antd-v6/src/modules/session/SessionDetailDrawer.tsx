import { useIntl } from '@umijs/max';
import { Alert, Descriptions, Drawer, Grid, Tag, Typography } from 'antd';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import { getOnlineSessionStatus } from './contract';
import { useOnlineSession } from './query';

function statusColor(status: ReturnType<typeof getOnlineSessionStatus>): string {
  if (status === 'active') return 'green';
  if (status === 'revoked') return 'red';
  return 'default';
}

export default function SessionDetailDrawer({
  id,
  open,
  onClose,
}: {
  id?: string;
  open: boolean;
  onClose: () => void;
}) {
  const intl = useIntl();
  const screens = Grid.useBreakpoint();
  const session = useOnlineSession(open ? id : undefined);
  const requestStatus = getRequestStatus(session.error);
  const authorizationFailure = requestStatus === 401 || requestStatus === 403;
  const formatDate = (value?: string) =>
    value
      ? new Intl.DateTimeFormat(intl.locale || 'zh-CN', {
          dateStyle: 'medium',
          timeStyle: 'medium',
        }).format(new Date(value))
      : '—';
  const status = session.data ? getOnlineSessionStatus(session.data) : undefined;

  return (
    <Drawer
      destroyOnHidden
      open={open}
      title={intl.formatMessage({ id: 'sessions.detail.title' })}
      size={screens.sm ? 560 : '100%'}
      onClose={onClose}
    >
      {session.isPending ? <PageLoading rows={8} /> : null}
      {session.isError && requestStatus === 403 ? (
        <PageForbidden message={intl.formatMessage({ id: 'sessions.forbidden' })} />
      ) : null}
      {session.isError && (requestStatus === 401 || !session.data) && requestStatus !== 403 ? (
        <PageError
          message={getRequestErrorMessage(session.error)}
          onRetry={() => void session.refetch()}
          retryLabel={intl.formatMessage({ id: 'actions.retry' })}
          title={intl.formatMessage({ id: 'states.loadError' })}
        />
      ) : null}
      {session.isError && session.data && !authorizationFailure ? (
        <Alert
          className="mb-4"
          showIcon
          title={intl.formatMessage({ id: 'sessions.detail.refreshFailed' })}
          type="warning"
        />
      ) : null}
      {session.data && !authorizationFailure ? (
        <Descriptions
          column={1}
          items={[
            {
              key: 'username',
              label: intl.formatMessage({ id: 'sessions.field.username' }),
              children: session.data.username,
            },
            {
              key: 'userID',
              label: intl.formatMessage({ id: 'sessions.field.userID' }),
              children: <Typography.Text copyable>{session.data.userID}</Typography.Text>,
            },
            {
              key: 'roleID',
              label: intl.formatMessage({ id: 'sessions.field.roleID' }),
              children: session.data.roleID || '—',
            },
            {
              key: 'status',
              label: intl.formatMessage({ id: 'sessions.field.status' }),
              children: status ? (
                <Tag color={statusColor(status)}>
                  {intl.formatMessage({ id: `sessions.status.${status}` })}
                </Tag>
              ) : (
                '—'
              ),
            },
            {
              key: 'ip',
              label: intl.formatMessage({ id: 'sessions.field.ip' }),
              children: session.data.ip || '—',
            },
            {
              key: 'userAgent',
              label: intl.formatMessage({ id: 'sessions.field.userAgent' }),
              children: <span className="break-all">{session.data.userAgent || '—'}</span>,
            },
            {
              key: 'loginAt',
              label: intl.formatMessage({ id: 'sessions.field.loginAt' }),
              children: formatDate(session.data.loginAt),
            },
            {
              key: 'lastSeenAt',
              label: intl.formatMessage({ id: 'sessions.field.lastSeenAt' }),
              children: formatDate(session.data.lastSeenAt),
            },
            {
              key: 'expiredAt',
              label: intl.formatMessage({ id: 'sessions.field.expiredAt' }),
              children: formatDate(session.data.expiredAt),
            },
            {
              key: 'revokedBy',
              label: intl.formatMessage({ id: 'sessions.field.revokedBy' }),
              children: session.data.revokedBy || '—',
            },
            {
              key: 'revokedAt',
              label: intl.formatMessage({ id: 'sessions.field.revokedAt' }),
              children: formatDate(session.data.revokedAt),
            },
            {
              key: 'revokeReason',
              label: intl.formatMessage({ id: 'sessions.field.revokeReason' }),
              children: session.data.revokeReason
                ? intl.formatMessage({ id: `sessions.reason.${session.data.revokeReason}` })
                : '—',
            },
          ]}
          size="small"
          bordered
        />
      ) : null}
    </Drawer>
  );
}
