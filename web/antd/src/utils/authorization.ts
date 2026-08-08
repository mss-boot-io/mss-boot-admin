export type AuthorizationIdentity = {
  role?: {
    root?: boolean;
  };
  permissions?: Record<string, unknown>;
};

export const ADMIN_PERMISSIONS = {
  appConfigControl: '/app-config/control',
  appConfigSecretsRead: '/app-config/secrets/read',
  appConfigSecretsWrite: '/app-config/secrets/write',
  storageUpload: '/storage/upload',
} as const;

export const isRootIdentity = (identity?: AuthorizationIdentity) => identity?.role?.root === true;

export const hasPermission = (identity: AuthorizationIdentity | undefined, permission?: string) => {
  if (!identity || !permission) {
    return false;
  }
  return isRootIdentity(identity) || identity.permissions?.[permission] === true;
};

export const hasEveryPermission = (
  identity: AuthorizationIdentity | undefined,
  permissions: readonly string[],
) =>
  isRootIdentity(identity) ||
  permissions.every((permission) => hasPermission(identity, permission));

export type PermissionMarkedRoute = {
  permission?: string;
  rootOnly?: boolean;
};

export const canAccessPermissionMarkedRoute = (
  identity: AuthorizationIdentity | undefined,
  route?: PermissionMarkedRoute,
) => (route?.rootOnly ? isRootIdentity(identity) : hasPermission(identity, route?.permission));
