import moment from 'moment';

export type SessionStatus = 'active' | 'revoked' | 'expired';

export function getSessionStatus(
  row: API.UserSession,
  now: moment.Moment = moment(),
): SessionStatus {
  if (row.revoked) return 'revoked';
  if (row.expiredAt && moment(row.expiredAt).isBefore(now)) return 'expired';
  return 'active';
}

export const STATUS_COLOR: Record<SessionStatus, string> = {
  active: 'green',
  revoked: 'red',
  expired: 'default',
};

export const STATUS_INTL_ID: Record<SessionStatus, string> = {
  active: 'pages.onlineSession.status.active',
  revoked: 'pages.onlineSession.status.revoked',
  expired: 'pages.onlineSession.status.expired',
};
