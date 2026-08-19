export { getRequestErrorMessage, getRequestStatus } from '../src/shared/api/errors';
export { canAccessRoute, hasPermission, isRootIdentity } from '../src/shared/auth/access';
export type {
  AuthorizedMenuItem,
  CurrentUser,
  InitialState,
} from '../src/shared/auth/types';
export { PageContainer } from '../src/shared/design-system/PageContainer';
export {
  PageEmpty,
  PageError,
  PageForbidden,
  PageLoading,
} from '../src/shared/design-system/PageState';
export { queryClient, queryKeys } from '../src/shared/query/client';
export type { RouteRegistration } from '../src/shared/routes/registry';
