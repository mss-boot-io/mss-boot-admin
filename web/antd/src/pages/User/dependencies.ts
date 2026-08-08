import { hasPermission, type AuthorizationIdentity } from '@/utils/authorization';

export const USER_DEPENDENCY_KEYS = ['roles', 'posts', 'departments'] as const;

export type UserDependencyKey = (typeof USER_DEPENDENCY_KEYS)[number];

export type UserDependencyAccess = Record<UserDependencyKey, boolean>;

export type UserDependencyLoaders = Record<UserDependencyKey, () => Promise<unknown>>;

type UserDependencyLoadResult = {
  values: Partial<Record<UserDependencyKey, unknown>>;
  errors: Partial<Record<UserDependencyKey, unknown>>;
};

const ALL_DEPENDENCIES: UserDependencyAccess = {
  roles: true,
  posts: true,
  departments: true,
};

const NO_DEPENDENCIES: UserDependencyAccess = {
  roles: false,
  posts: false,
  departments: false,
};

export const resolveUserDependencyAccess = (
  identity: AuthorizationIdentity | undefined,
  options: { isForm: boolean; isMobile: boolean },
): UserDependencyAccess => {
  if (options.isForm) {
    return { ...ALL_DEPENDENCIES };
  }
  if (options.isMobile) {
    return { ...NO_DEPENDENCIES };
  }
  return {
    roles: hasPermission(identity, '/role'),
    posts: hasPermission(identity, '/posts'),
    departments: hasPermission(identity, '/departments'),
  };
};

/**
 * List pages use dependency data only to enrich IDs with labels, so failures
 * are returned alongside successful values. Forms use strict mode because
 * incomplete option data could produce an invalid update.
 */
export const loadUserDependencies = async (
  access: UserDependencyAccess,
  loaders: UserDependencyLoaders,
  strict: boolean,
): Promise<UserDependencyLoadResult> => {
  const requestedKeys = USER_DEPENDENCY_KEYS.filter((key) => access[key]);
  const settled = await Promise.allSettled(requestedKeys.map((key) => loaders[key]()));
  const values: UserDependencyLoadResult['values'] = {};
  const errors: UserDependencyLoadResult['errors'] = {};

  settled.forEach((result, index) => {
    const key = requestedKeys[index];
    if (result.status === 'fulfilled') {
      values[key] = result.value;
    } else {
      errors[key] = result.reason;
    }
  });

  const firstFailure = settled.find(
    (result): result is PromiseRejectedResult => result.status === 'rejected',
  );
  if (strict && firstFailure) {
    throw firstFailure.reason;
  }

  return { values, errors };
};
