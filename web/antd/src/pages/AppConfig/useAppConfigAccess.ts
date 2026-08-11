import { useModel } from '@umijs/max';
import { ADMIN_PERMISSIONS, hasEveryPermission, hasPermission } from '@/utils/authorization';

export const APP_CONFIG_SECRET_FIELDS = {
  email: ['password'],
  security: ['githubClientSecret', 'larkAppSecret'],
} as const;

export type AppConfigSecretGroup = keyof typeof APP_CONFIG_SECRET_FIELDS;

export type AppConfigSecretPayloadAccess = {
  canReadSecrets: boolean;
  canWriteSecrets: boolean;
};

export const omitAppConfigSecrets = <T extends Record<string, any>>(
  group: AppConfigSecretGroup,
  values: T,
): T => {
  const next = { ...values };
  APP_CONFIG_SECRET_FIELDS[group].forEach((key) => delete next[key]);
  return next;
};

/**
 * Keeps blind secret rotation safe: when the current value cannot be read, only a
 * newly entered non-empty string may replace it. Read-capable editors retain the
 * existing explicit-clear behavior.
 */
export const prepareAppConfigSecretPayload = <T extends Record<string, any>>(
  group: AppConfigSecretGroup,
  values: T,
  access: AppConfigSecretPayloadAccess,
): T => {
  const next = { ...values };

  APP_CONFIG_SECRET_FIELDS[group].forEach((key) => {
    if (!access.canWriteSecrets) {
      delete next[key];
      return;
    }

    const value = next[key];
    if (!access.canReadSecrets && (typeof value !== 'string' || value.trim().length === 0)) {
      delete next[key];
    }
  });

  return next;
};

export const useAppConfigAccess = () => {
  const { initialState } = useModel('@@initialState');
  const currentUser = initialState?.currentUser;
  const canWrite = hasPermission(currentUser, ADMIN_PERMISSIONS.appConfigControl);

  return {
    canWrite,
    canUpload: hasEveryPermission(currentUser, [
      ADMIN_PERMISSIONS.appConfigControl,
      ADMIN_PERMISSIONS.storageUpload,
    ]),
    canReadSecrets: hasPermission(currentUser, ADMIN_PERMISSIONS.appConfigSecretsRead),
    canWriteSecrets:
      canWrite && hasPermission(currentUser, ADMIN_PERMISSIONS.appConfigSecretsWrite),
  };
};
