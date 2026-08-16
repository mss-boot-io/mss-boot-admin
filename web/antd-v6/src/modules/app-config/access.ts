import { useModel } from '@umijs/max';
import { hasPermission } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';

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
