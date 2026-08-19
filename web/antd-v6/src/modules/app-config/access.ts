import { hasPermission } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { useModel } from '@umijs/max';

export function useAppConfigAccess() {
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const user = initialState?.currentUser;
  const canWrite = hasPermission(user, '/app-config/control');
  return {
    canWrite,
    canUpload: canWrite && hasPermission(user, '/storage/upload'),
    canReadSecrets: hasPermission(user, '/app-config/secrets/read'),
    canWriteSecrets: canWrite && hasPermission(user, '/app-config/secrets/write'),
  };
}
