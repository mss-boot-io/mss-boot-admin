import { describe, expect, it } from 'vitest';
import {
  getOnlineSessionStatus,
  OnlineSessionContractError,
  parseOnlineSession,
  parseOnlineSessionPage,
  parseRevokeUserResult,
} from './contract';

function session() {
  return {
    id: 'session-1',
    userID: 'user-1',
    username: 'alice',
    roleID: 'role-1',
    loginAt: '2026-08-15T00:00:00Z',
    lastSeenAt: '2026-08-15T01:00:00Z',
    expiredAt: '2026-08-15T02:00:00Z',
    ip: '127.0.0.1',
    userAgent: 'Browser',
    revoked: false,
  };
}

describe('online session contracts', () => {
  it('parses bounded page data and derives active, expired, and revoked states', () => {
    const page = parseOnlineSessionPage({ data: [session()], total: 1, current: 1, pageSize: 20 });
    const first = page.data[0];
    expect(first?.id).toBe('session-1');
    if (!first) throw new Error('Expected a parsed online session');
    expect(getOnlineSessionStatus(first, Date.parse('2026-08-15T01:30:00Z'))).toBe('active');
    expect(getOnlineSessionStatus(first, Date.parse('2026-08-15T03:00:00Z'))).toBe('expired');
    expect(getOnlineSessionStatus({ ...first, revoked: true })).toBe('revoked');
  });

  it('rejects malformed dates, unknown reasons, and non-boolean revocation state', () => {
    expect(() => parseOnlineSession({ ...session(), expiredAt: 'not-a-date' })).toThrow(
      OnlineSessionContractError,
    );
    expect(() => parseOnlineSession({ ...session(), revokeReason: 'unknown' })).toThrow(
      /revokeReason/,
    );
    expect(() => parseOnlineSession({ ...session(), revoked: 0 })).toThrow(/revoked/);
  });

  it('rejects unbounded, duplicated, and request-mismatched page metadata', () => {
    expect(() => parseOnlineSessionPage({ data: [], total: 0, current: 1, pageSize: 101 })).toThrow(
      /metadata/,
    );
    expect(() =>
      parseOnlineSessionPage({
        data: [session(), session()],
        total: 2,
        current: 1,
        pageSize: 20,
      }),
    ).toThrow(/metadata/);
    expect(() =>
      parseOnlineSessionPage(
        { data: [session()], total: 1, current: 2, pageSize: 20 },
        { current: 1, pageSize: 20 },
      ),
    ).toThrow(/metadata/);
  });

  it('binds batch revoke responses to the requested user', () => {
    expect(parseRevokeUserResult({ affected: 2, userID: 'user-1' }, 'user-1')).toEqual({
      affected: 2,
      userID: 'user-1',
    });
    expect(() => parseRevokeUserResult({ affected: 2, userID: 'user-2' }, 'user-1')).toThrow(
      /another user/,
    );
  });
});
