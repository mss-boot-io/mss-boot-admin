import { getDepartmentsId } from '@/services/admin/department';
import { getPostsId } from '@/services/admin/post';
import { getUserUserInfo } from '@/services/admin/user';

export interface AccountCenterDetails {
  departmentInfo?: API.Department;
  postInfo?: API.Post;
}

export async function getAccountCenterUser(
  initialUser: API.User | undefined,
  signal?: AbortSignal,
): Promise<API.User> {
  return initialUser ?? getUserUserInfo({ signal });
}

/**
 * Load independent account relations together. Passing the route's abort signal
 * lets a route change stop requests that can no longer affect the screen.
 */
export async function getAccountCenterDetails(
  userInfo: Pick<API.User, 'department' | 'post'> | undefined,
  signal?: AbortSignal,
): Promise<AccountCenterDetails> {
  const requestOptions = signal ? { signal } : undefined;
  const [departmentInfo, postInfo] = await Promise.all([
    userInfo?.department?.id
      ? getDepartmentsId({ id: userInfo.department.id }, requestOptions)
      : Promise.resolve(undefined),
    userInfo?.post?.id
      ? getPostsId({ id: userInfo.post.id }, requestOptions)
      : Promise.resolve(undefined),
  ]);

  return { departmentInfo, postInfo };
}
