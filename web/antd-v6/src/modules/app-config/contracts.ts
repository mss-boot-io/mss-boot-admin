export type AppConfigGroup = 'base' | 'security' | 'storage' | 'email';

export interface BaseAppConfig {
  websiteName: string;
  websiteDescription: string;
  websiteLogo: string;
  websiteRecordNumber: string;
  websiteCopyRight: string;
}

export interface SecurityAppConfig {
  registerEnabled: boolean;
  phoneEnabled: boolean;
  emailEnabled: boolean;
  githubEnabled: boolean;
  githubAllowGroup: string;
  githubBrowserSessionClientId: string;
  githubBrowserSessionClientSecret?: string;
  githubBrowserSessionRedirectURI: string;
  githubBrowserSessionScope: string;
  larkEnabled: boolean;
  larkBrowserSessionAppId: string;
  larkBrowserSessionAppSecret?: string;
  larkBrowserSessionRedirectURI: string;
}

export interface StorageAppConfig {
  maxSize: number;
  allowedTypes: string;
}

export interface EmailAppConfig {
  smtpHost: string;
  smtpPort: number;
  username: string;
  password?: string;
}

export const APP_CONFIG_SECRET_KEYS = [
  'githubBrowserSessionClientSecret',
  'larkBrowserSessionAppSecret',
  'password',
] as const;

export type AppConfigSecretKey = (typeof APP_CONFIG_SECRET_KEYS)[number];
export type SecurityAppConfigSecretKey = Exclude<AppConfigSecretKey, 'password'>;

export interface ProtectedAppConfig<T> {
  values: T;
  configuredSecrets: ReadonlySet<AppConfigSecretKey>;
}

export class AppConfigContractError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AppConfigContractError';
  }
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

const stringValue = (value: unknown): string => (typeof value === 'string' ? value : '');

const booleanValue = (value: unknown): boolean => value === true || value === 'true' || value === 1;

const integerValue = (value: unknown, fallback: number): number => {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isSafeInteger(parsed) ? parsed : fallback;
};

function requireGroup(value: unknown): Record<string, unknown> {
  if (!isRecord(value)) throw new AppConfigContractError('Application config group is invalid');
  return value;
}

export function parseBaseAppConfig(value: unknown): BaseAppConfig {
  const group = requireGroup(value);
  return {
    websiteName: stringValue(group.websiteName),
    websiteDescription: stringValue(group.websiteDescription),
    websiteLogo: stringValue(group.websiteLogo),
    websiteRecordNumber: stringValue(group.websiteRecordNumber),
    websiteCopyRight: stringValue(group.websiteCopyRight),
  };
}

const securitySecretKeys = new Set<SecurityAppConfigSecretKey>([
  'githubBrowserSessionClientSecret',
  'larkBrowserSessionAppSecret',
]);

export function parseSecurityAppConfig(value: unknown): ProtectedAppConfig<SecurityAppConfig> {
  const group = requireGroup(value);
  const configuredSecrets = new Set<AppConfigSecretKey>();
  for (const key of securitySecretKeys) {
    if (stringValue(group[key])) configuredSecrets.add(key);
  }
  return {
    values: {
      registerEnabled: booleanValue(group.registerEnabled),
      phoneEnabled: booleanValue(group.phoneEnabled),
      emailEnabled: booleanValue(group.emailEnabled),
      githubEnabled: booleanValue(group.githubEnabled),
      githubAllowGroup: stringValue(group.githubAllowGroup),
      githubBrowserSessionClientId: stringValue(group.githubBrowserSessionClientId),
      githubBrowserSessionRedirectURI: stringValue(group.githubBrowserSessionRedirectURI),
      githubBrowserSessionScope: stringValue(group.githubBrowserSessionScope),
      larkEnabled: booleanValue(group.larkEnabled),
      larkBrowserSessionAppId: stringValue(group.larkBrowserSessionAppId),
      larkBrowserSessionRedirectURI: stringValue(group.larkBrowserSessionRedirectURI),
    },
    configuredSecrets,
  };
}

export function parseStorageAppConfig(value: unknown): StorageAppConfig {
  const group = requireGroup(value);
  return {
    maxSize: integerValue(group.maxSize, 10 * 1024 * 1024),
    allowedTypes: stringValue(group.allowedTypes),
  };
}

export function parseEmailAppConfig(value: unknown): ProtectedAppConfig<EmailAppConfig> {
  const group = requireGroup(value);
  const configuredSecrets = new Set<AppConfigSecretKey>();
  if (stringValue(group.password)) configuredSecrets.add('password');
  return {
    values: {
      smtpHost: stringValue(group.smtpHost),
      smtpPort: integerValue(group.smtpPort, 587),
      username: stringValue(group.username),
    },
    configuredSecrets,
  };
}

function trimmed(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined;
  const result = value.trim();
  return result || undefined;
}

export function serializeBaseAppConfig(value: BaseAppConfig): Record<string, unknown> {
  return {
    websiteName: value.websiteName.trim(),
    websiteDescription: value.websiteDescription.trim(),
    websiteLogo: value.websiteLogo.trim(),
    websiteRecordNumber: value.websiteRecordNumber.trim(),
    websiteCopyRight: value.websiteCopyRight.trim(),
  };
}

export function serializeSecurityAppConfig(
  value: SecurityAppConfig,
  canWriteSecrets: boolean,
): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    registerEnabled: value.registerEnabled,
    phoneEnabled: value.phoneEnabled,
    emailEnabled: value.emailEnabled,
    githubEnabled: value.githubEnabled,
    githubAllowGroup: value.githubAllowGroup.trim(),
    githubBrowserSessionClientId: value.githubBrowserSessionClientId.trim(),
    githubBrowserSessionRedirectURI: value.githubBrowserSessionRedirectURI.trim(),
    githubBrowserSessionScope: value.githubBrowserSessionScope.trim(),
    larkEnabled: value.larkEnabled,
    larkBrowserSessionAppId: value.larkBrowserSessionAppId.trim(),
    larkBrowserSessionRedirectURI: value.larkBrowserSessionRedirectURI.trim(),
  };
  if (canWriteSecrets) {
    for (const key of securitySecretKeys) {
      const secret = trimmed(value[key as keyof SecurityAppConfig]);
      if (secret) payload[key] = secret;
    }
  }
  return payload;
}

export function serializeStorageAppConfig(value: StorageAppConfig): Record<string, unknown> {
  return { maxSize: value.maxSize, allowedTypes: value.allowedTypes.trim() };
}

export function serializeEmailAppConfig(
  value: EmailAppConfig,
  canWriteSecrets: boolean,
): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    smtpHost: value.smtpHost.trim(),
    smtpPort: value.smtpPort,
    username: value.username.trim(),
  };
  const password = trimmed(value.password);
  if (canWriteSecrets && password) payload.password = password;
  return payload;
}

export function parseStorageUpload(value: unknown): string {
  const result = requireGroup(value);
  const url = stringValue(result.url).trim();
  if (!url) throw new AppConfigContractError('Storage upload URL is missing');
  return url;
}
