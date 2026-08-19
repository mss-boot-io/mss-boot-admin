import { canAccessRoute, isRootIdentity } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';

export default function access(initialState?: InitialState) {
  const currentUser = initialState?.currentUser;
  return {
    canAdmin: isRootIdentity(currentUser),
    canAccessRoute: (route?: { permission?: string; rootOnly?: boolean }) =>
      canAccessRoute(currentUser, route),
  };
}
