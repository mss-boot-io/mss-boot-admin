import type { CurrentUser } from '@mss-admin-core/shared/auth/types';

export type OAuthProvider = 'github' | 'lark';
export type OAuthIntent = 'login' | 'binding' | 'reauthentication';

export interface ProfileUpdate {
  address?: string;
  avatar?: string;
  city?: string;
  country?: string;
  group?: string;
  name?: string;
  phone?: string;
  profile?: string;
  province?: string;
  signature?: string;
  tags?: string[];
  title?: string;
}

export interface AccessTokenSummary {
  id: string;
  userID: string;
  fingerprint?: string;
  expiredAt: string;
  revoked: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface AccessTokenSecret extends AccessTokenSummary {
  token: string;
}

export interface OAuthBinding {
  id?: string;
  provider: OAuthProvider;
  displayName?: string;
  picture?: string;
  email?: string;
}

export interface OAuthAuthorizeAttempt {
  authorizeURL: string;
  attemptID: string;
  expiresAt: string;
}

export interface OAuthCallbackOutcome {
  provider: OAuthProvider;
  intent: OAuthIntent;
  attemptID: string;
}

export interface AccountSecurityStatus {
  hasLocalPassword: boolean;
  recentAuthentication: boolean;
  recentAuthenticationExpiresAt?: string;
  reauthenticationLockedUntil?: string;
}

export type NotificationSettingKey = 'password' | 'system' | 'todo' | 'email';
export type NotificationSettings = Readonly<Record<NotificationSettingKey, boolean>>;

export class AccountContractError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AccountContractError';
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function requiredString(value: unknown, field: string): string {
  if (typeof value !== 'string' || !value.trim()) {
    throw new AccountContractError(`${field} must be a non-empty string`);
  }
  return value.trim();
}

function optionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined;
}

function provider(value: unknown): OAuthProvider {
  if (value === 'github' || value === 'lark') return value;
  throw new AccountContractError('OAuth provider is unsupported');
}

export function buildProfileUpdate(values: Partial<CurrentUser>): ProfileUpdate {
  return {
    address: values.address,
    avatar: values.avatar,
    city: values.city,
    country: values.country,
    group: values.group,
    name: values.name,
    phone: values.phone,
    profile: values.profile,
    province: values.province,
    signature: values.signature,
    tags: values.tags ? [...values.tags] : undefined,
    title: values.title,
  };
}

export function parseAvatarResponse(value: unknown): string {
  if (!isRecord(value)) throw new AccountContractError('Avatar response must be an object');
  return requiredString(value.avatar, 'avatar');
}

export function parseAccessTokenSummary(value: unknown): AccessTokenSummary {
  if (!isRecord(value)) throw new AccountContractError('Access token summary must be an object');
  return {
    id: requiredString(value.id, 'id'),
    userID: requiredString(value.userID, 'userID'),
    fingerprint: optionalString(value.fingerprint),
    expiredAt: requiredString(value.expiredAt, 'expiredAt'),
    revoked: value.revoked === true,
    createdAt: optionalString(value.createdAt),
    updatedAt: optionalString(value.updatedAt),
  };
}

export function parseAccessTokenPage(value: unknown): AccessTokenSummary[] {
  if (!isRecord(value) || !Array.isArray(value.data)) {
    throw new AccountContractError('Access token page must contain a data array');
  }
  // Parsing into a fresh exact shape intentionally strips a raw `token` field
  // if an older mixed-version server accidentally serializes one in a list.
  return value.data.map(parseAccessTokenSummary);
}

export function parseAccessTokenSecret(value: unknown): AccessTokenSecret {
  if (!isRecord(value)) throw new AccountContractError('Access token secret must be an object');
  return {
    ...parseAccessTokenSummary(value),
    token: requiredString(value.token, 'token'),
  };
}

export function parseOAuthBindings(value: unknown): OAuthBinding[] {
  if (!Array.isArray(value)) throw new AccountContractError('OAuth bindings must be an array');
  return value.map((entry) => {
    if (!isRecord(entry)) throw new AccountContractError('OAuth binding must be an object');
    return {
      id: optionalString(entry.id),
      provider: provider(entry.type),
      displayName:
        optionalString(entry.name) ??
        optionalString(entry.nickname) ??
        optionalString(entry.preferred_username),
      picture: optionalString(entry.picture),
      email: optionalString(entry.email),
    };
  });
}

export function parseOAuthAuthorizeAttempt(value: unknown): OAuthAuthorizeAttempt {
  if (!isRecord(value)) {
    throw new AccountContractError('OAuth authorization response must be an object');
  }
  const authorizeURL = requiredString(value.authorizeURL, 'authorizeURL');
  let parsedURL: URL;
  try {
    parsedURL = new URL(authorizeURL);
  } catch {
    throw new AccountContractError('OAuth authorization URL is invalid');
  }
  if (parsedURL.protocol !== 'https:' && parsedURL.protocol !== 'http:') {
    throw new AccountContractError('OAuth authorization URL protocol is invalid');
  }
  return {
    authorizeURL: parsedURL.toString(),
    attemptID: requiredString(value.attemptID, 'attemptID'),
    expiresAt: requiredString(value.expiresAt, 'expiresAt'),
  };
}

export function parseOAuthCallbackOutcome(
  value: unknown,
  expectedProvider: string,
): OAuthCallbackOutcome {
  if (!isRecord(value)) throw new AccountContractError('OAuth callback must be an object');
  if (value.code !== 200) {
    throw new AccountContractError('OAuth callback did not establish a successful session');
  }
  const callbackProvider = provider(value.provider);
  if (callbackProvider !== expectedProvider) {
    throw new AccountContractError('OAuth callback provider does not match the route');
  }
  if (
    value.intent !== 'login' &&
    value.intent !== 'binding' &&
    value.intent !== 'reauthentication'
  ) {
    throw new AccountContractError('OAuth callback intent is invalid');
  }
  return {
    provider: callbackProvider,
    intent: value.intent,
    attemptID: requiredString(value.attemptID, 'attemptID'),
  };
}

export function parseAccountSecurityStatus(value: unknown): AccountSecurityStatus {
  if (!isRecord(value)) throw new AccountContractError('Account security must be an object');
  return {
    hasLocalPassword: value.hasLocalPassword === true,
    recentAuthentication: value.recentAuthentication === true,
    recentAuthenticationExpiresAt: optionalString(value.recentAuthenticationExpiresAt),
    reauthenticationLockedUntil: optionalString(value.reauthenticationLockedUntil),
  };
}

export function parsePasswordChangeResponse(value: unknown): { signedOut: true } {
  if (!isRecord(value) || value.signedOut !== true) {
    throw new AccountContractError('Password change did not revoke the browser session');
  }
  return { signedOut: true };
}

const notificationKeys: readonly NotificationSettingKey[] = ['password', 'system', 'todo', 'email'];

export function parseNotificationSettings(value: unknown): NotificationSettings {
  if (!isRecord(value)) throw new AccountContractError('Notification settings must be an object');
  return Object.freeze(
    Object.fromEntries(
      notificationKeys.map((key) => [key, value[key] === true || value[key] === 'true']),
    ) as Record<NotificationSettingKey, boolean>,
  );
}

export function serializeNotificationSetting(
  key: NotificationSettingKey,
  enabled: boolean,
): { data: Partial<Record<NotificationSettingKey, boolean>> } {
  return { data: { [key]: enabled } };
}
