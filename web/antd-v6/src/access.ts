import { canAccessRoute, isRootIdentity } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';

export default function access(initialState?: InitialState) {
  const currentUser = initialState?.currentUser;
  return {
    canAdmin: isRootIdentity(currentUser),
    canAccessRoute: (route?: { permission?: string; rootOnly?: boolean }) =>
      canAccessRoute(currentUser, route),
  };
}
