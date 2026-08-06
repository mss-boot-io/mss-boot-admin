import { getDepartmentsId } from '@/services/admin/department';
import { getPostsId } from '@/services/admin/post';
import { getUserUserInfo } from '@/services/admin/user';
import { getAccountCenterDetails, getAccountCenterUser } from './details';

jest.mock('@/services/admin/department', () => ({
  getDepartmentsId: jest.fn(),
}));

jest.mock('@/services/admin/post', () => ({
  getPostsId: jest.fn(),
}));

jest.mock('@/services/admin/user', () => ({
  getUserUserInfo: jest.fn(),
}));

const mockGetDepartmentsId = getDepartmentsId as jest.Mock;
const mockGetPostsId = getPostsId as jest.Mock;
const mockGetUserUserInfo = getUserUserInfo as jest.Mock;

describe('getAccountCenterDetails', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('starts department and post requests in parallel and forwards cancellation', async () => {
    let resolveDepartment: ((value: API.Department) => void) | undefined;
    let resolvePost: ((value: API.Post) => void) | undefined;
    const departmentPromise = new Promise<API.Department>((resolve) => {
      resolveDepartment = resolve;
    });
    const postPromise = new Promise<API.Post>((resolve) => {
      resolvePost = resolve;
    });
    const controller = new AbortController();

    mockGetDepartmentsId.mockReturnValue(departmentPromise);
    mockGetPostsId.mockReturnValue(postPromise);

    const detailsPromise = getAccountCenterDetails(
      {
        department: { id: 'department-1' },
        post: { id: 'post-1' },
      },
      controller.signal,
    );

    expect(mockGetDepartmentsId).toHaveBeenCalledWith(
      { id: 'department-1' },
      { signal: controller.signal },
    );
    expect(mockGetPostsId).toHaveBeenCalledWith({ id: 'post-1' }, { signal: controller.signal });

    if (!resolveDepartment || !resolvePost) {
      throw new Error('Expected both relation requests to be pending');
    }

    resolveDepartment({ id: 'department-1', name: 'Engineering' });
    resolvePost({ id: 'post-1', name: 'Developer' });

    await expect(detailsPromise).resolves.toEqual({
      departmentInfo: { id: 'department-1', name: 'Engineering' },
      postInfo: { id: 'post-1', name: 'Developer' },
    });
  });

  it('uses the initial-state user without issuing another userInfo request', async () => {
    const initialUser: API.User = { id: 'user-1', username: 'admin' };

    await expect(getAccountCenterUser(initialUser)).resolves.toBe(initialUser);

    expect(mockGetUserUserInfo).not.toHaveBeenCalled();
  });

  it('falls back to userInfo only when the initial state is unavailable', async () => {
    const controller = new AbortController();
    mockGetUserUserInfo.mockResolvedValue({ id: 'user-1', username: 'admin' });

    await expect(getAccountCenterUser(undefined, controller.signal)).resolves.toEqual({
      id: 'user-1',
      username: 'admin',
    });

    expect(mockGetUserUserInfo).toHaveBeenCalledWith({ signal: controller.signal });
  });

  it('does not make relation requests without relation IDs', async () => {
    await expect(getAccountCenterDetails({})).resolves.toEqual({
      departmentInfo: undefined,
      postInfo: undefined,
    });

    expect(mockGetDepartmentsId).not.toHaveBeenCalled();
    expect(mockGetPostsId).not.toHaveBeenCalled();
  });
});
