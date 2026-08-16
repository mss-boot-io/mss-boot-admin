export const ONLINE_SESSION_PAGE_SIZES = [20, 50, 100] as const;
export const MAX_ONLINE_SESSION_PAGE_SIZE = 100;

export type OnlineSessionStatus = 'active' | 'revoked' | 'expired';
export type OnlineSessionStatusFilter = OnlineSessionStatus | 'all';
export type SessionRevokeReason = 'logout' | 'force-by-session' | 'force-by-user';

export interface OnlineSession {
  id: string;
  userID: string;
  username: string;
  roleID?: string;
  loginAt: string;
  lastSeenAt: string;
  expiredAt: string;
  ip?: string;
  userAgent?: string;
  revoked: boolean;
  revokedAt?: string;
  revokedBy?: string;
  revokeReason?: SessionRevokeReason;
}

export interface OnlineSessionListParams {
  current: number;
  pageSize: (typeof ONLINE_SESSION_PAGE_SIZES)[number];
  status: OnlineSessionStatusFilter;
  userID?: string;
  username?: string;
  ip?: string;
}

export interface OnlineSessionPage {
  data: OnlineSession[];
  total: number;
  current: number;
  pageSize: number;
}

export interface RevokeUserResult {
  affected: number;
  userID: string;
}

export function isOnlineSessionPageSize(
  value: number,
): value is OnlineSessionListParams['pageSize'] {
  return (ONLINE_SESSION_PAGE_SIZES as readonly number[]).includes(value);
}

export class OnlineSessionContractError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'OnlineSessionContractError';
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringField(
  value: Record<string, unknown>,
  key: string,
  options: { optional?: boolean; maxLength?: number } = {},
): string | undefined {
  const candidate = value[key];
  if (candidate === undefined || candidate === null || candidate === '') {
    if (options.optional) return undefined;
    throw new OnlineSessionContractError(`Online session field ${key} is invalid`);
  }
  if (
    typeof candidate !== 'string' ||
    !candidate.trim() ||
    (options.maxLength !== undefined && candidate.length > options.maxLength)
  ) {
    throw new OnlineSessionContractError(`Online session field ${key} is invalid`);
  }
  return candidate.trim();
}

function dateField(
  value: Record<string, unknown>,
  key: string,
  optional = false,
): string | undefined {
  const candidate = stringField(value, key, { optional, maxLength: 64 });
  if (candidate === undefined) return undefined;
  if (!Number.isFinite(Date.parse(candidate))) {
    throw new OnlineSessionContractError(`Online session field ${key} is invalid`);
  }
  return candidate;
}

function integerField(value: Record<string, unknown>, key: string, min: number): number {
  const candidate = value[key];
  if (typeof candidate !== 'number' || !Number.isSafeInteger(candidate) || candidate < min) {
    throw new OnlineSessionContractError(`Online session field ${key} is invalid`);
  }
  return candidate;
}

function revokeReasonField(value: Record<string, unknown>): SessionRevokeReason | undefined {
  const candidate = stringField(value, 'revokeReason', { optional: true, maxLength: 32 });
  if (candidate === undefined) return undefined;
  if (!['logout', 'force-by-session', 'force-by-user'].includes(candidate)) {
    throw new OnlineSessionContractError('Online session field revokeReason is invalid');
  }
  return candidate as SessionRevokeReason;
}

export function parseOnlineSession(value: unknown): OnlineSession {
  if (!isRecord(value)) throw new OnlineSessionContractError('Online session must be an object');
  if (typeof value.revoked !== 'boolean') {
    throw new OnlineSessionContractError('Online session field revoked is invalid');
  }
  return {
    id: stringField(value, 'id', { maxLength: 128 }) as string,
    userID: stringField(value, 'userID', { maxLength: 128 }) as string,
    username: stringField(value, 'username', { maxLength: 255 }) as string,
    roleID: stringField(value, 'roleID', { optional: true, maxLength: 128 }),
    loginAt: dateField(value, 'loginAt') as string,
    lastSeenAt: dateField(value, 'lastSeenAt') as string,
    expiredAt: dateField(value, 'expiredAt') as string,
    ip: stringField(value, 'ip', { optional: true, maxLength: 64 }),
    userAgent: stringField(value, 'userAgent', { optional: true, maxLength: 1_000 }),
    revoked: value.revoked,
    revokedAt: dateField(value, 'revokedAt', true),
    revokedBy: stringField(value, 'revokedBy', { optional: true, maxLength: 128 }),
    revokeReason: revokeReasonField(value),
  };
}

export function parseOnlineSessionPage(
  value: unknown,
  expected?: Pick<OnlineSessionListParams, 'current' | 'pageSize'>,
): OnlineSessionPage {
  if (!isRecord(value) || !Array.isArray(value.data)) {
    throw new OnlineSessionContractError('Online session page is invalid');
  }
  const data = value.data.map(parseOnlineSession);
  const total = integerField(value, 'total', 0);
  const current = integerField(value, 'current', 1);
  const pageSize = integerField(value, 'pageSize', 1);
  if (
    pageSize > MAX_ONLINE_SESSION_PAGE_SIZE ||
    data.length > pageSize ||
    data.length > total ||
    new Set(data.map((session) => session.id)).size !== data.length ||
    (expected && (current !== expected.current || pageSize !== expected.pageSize))
  ) {
    throw new OnlineSessionContractError('Online session page metadata is invalid');
  }
  return { data, total, current, pageSize };
}

export function parseRevokeUserResult(value: unknown, expectedUserID: string): RevokeUserResult {
  if (!isRecord(value)) throw new OnlineSessionContractError('Revoke result is invalid');
  const userID = stringField(value, 'userID', { maxLength: 128 }) as string;
  if (userID !== expectedUserID) {
    throw new OnlineSessionContractError('Revoke result belongs to another user');
  }
  return { affected: integerField(value, 'affected', 0), userID };
}

export function getOnlineSessionStatus(
  session: Pick<OnlineSession, 'revoked' | 'expiredAt'>,
  now = Date.now(),
): OnlineSessionStatus {
  if (session.revoked) return 'revoked';
  return Date.parse(session.expiredAt) <= now ? 'expired' : 'active';
}
