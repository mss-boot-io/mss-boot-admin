import { describe, expect, it } from 'vitest';
import {
  AccountContractError,
  buildProfileUpdate,
  parseAccessTokenPage,
  parseAccessTokenSecret,
  parseAccountSecurityStatus,
  parseNotificationSettings,
  parseOAuthBindings,
  parseOAuthCallbackOutcome,
} from './contracts';

describe('account contracts', () => {
  it('builds only the supported self-service profile fields', () => {
    expect(
      buildProfileUpdate({
        id: 'user-1',
        username: 'immutable',
        email: 'identity@example.com',
        name: 'Profile name',
        phone: '123',
        tags: ['one'],
        permissions: {},
      }),
    ).toEqual({
      address: undefined,
      avatar: undefined,
      city: undefined,
      country: undefined,
      group: undefined,
      name: 'Profile name',
      phone: '123',
      profile: undefined,
      province: undefined,
      signature: undefined,
      tags: ['one'],
      title: undefined,
    });
  });

  it('strips legacy raw secrets from token list resources', () => {
    const result = parseAccessTokenPage({
      data: [
        {
          id: 'pat-1',
          userID: 'user-1',
          fingerprint: 'abcdef123456',
          expiredAt: '2027-01-01T00:00:00Z',
          revoked: false,
          token: 'must-not-survive',
        },
      ],
    });
    expect(result).toEqual([
      {
        id: 'pat-1',
        userID: 'user-1',
        fingerprint: 'abcdef123456',
        expiredAt: '2027-01-01T00:00:00Z',
        revoked: false,
        createdAt: undefined,
        updatedAt: undefined,
      },
    ]);
    expect(result[0]).not.toHaveProperty('token');
  });

  it('requires a one-time token in create and rotate responses', () => {
    expect(() =>
      parseAccessTokenSecret({
        id: 'pat-1',
        userID: 'user-1',
        expiredAt: '2027-01-01T00:00:00Z',
        revoked: false,
      }),
    ).toThrow(AccountContractError);
  });

  it('normalizes only supported OAuth providers and notification booleans', () => {
    expect(parseOAuthBindings([{ id: 'one', type: 'github', nickname: 'Octocat' }])).toEqual([
      {
        id: 'one',
        provider: 'github',
        displayName: 'Octocat',
        picture: undefined,
        email: undefined,
      },
    ]);
    expect(() => parseOAuthBindings([{ type: 'unknown' }])).toThrow(AccountContractError);
    expect(parseNotificationSettings({ password: 'true', system: false, todo: 'false' })).toEqual({
      password: true,
      system: false,
      todo: false,
      email: false,
    });
  });

  it('binds OAuth callback intent and provider to the server response', () => {
    expect(
      parseOAuthCallbackOutcome(
        { code: 200, provider: 'lark', intent: 'binding', attemptID: 'attempt-1' },
        'lark',
      ),
    ).toEqual({ provider: 'lark', intent: 'binding', attemptID: 'attempt-1' });
    expect(() =>
      parseOAuthCallbackOutcome(
        { code: 200, provider: 'github', intent: 'binding', attemptID: 'attempt-1' },
        'lark',
      ),
    ).toThrow(AccountContractError);
    expect(() =>
      parseOAuthCallbackOutcome(
        { code: 201, provider: 'lark', intent: 'binding', attemptID: 'attempt-1' },
        'lark',
      ),
    ).toThrow(AccountContractError);
    expect(
      parseOAuthCallbackOutcome(
        {
          code: 200,
          provider: 'github',
          intent: 'reauthentication',
          attemptID: 'attempt-2',
        },
        'github',
      ),
    ).toEqual({
      provider: 'github',
      intent: 'reauthentication',
      attemptID: 'attempt-2',
    });
  });

  it('projects only safe account security metadata', () => {
    expect(
      parseAccountSecurityStatus({
        hasLocalPassword: true,
        recentAuthentication: true,
        recentAuthenticationExpiresAt: '2026-08-16T04:00:00Z',
        passwordHash: 'must-not-survive',
        salt: 'must-not-survive',
      }),
    ).toEqual({
      hasLocalPassword: true,
      recentAuthentication: true,
      recentAuthenticationExpiresAt: '2026-08-16T04:00:00Z',
      reauthenticationLockedUntil: undefined,
    });
  });
});
