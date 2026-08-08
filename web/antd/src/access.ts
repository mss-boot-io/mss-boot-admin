import { canAccessPermissionMarkedRoute, isRootIdentity } from '@/utils/authorization';

/** @see https://umijs.org/zh-CN/plugins/plugin-access */
export default function access(initialState: { currentUser?: API.User } | undefined) {
  const currentUser = initialState?.currentUser;
  return {
    canAdmin: isRootIdentity(currentUser),
    canAccessRoute: (route?: { permission?: string; rootOnly?: boolean }) =>
      canAccessPermissionMarkedRoute(currentUser, route),
  };
}
