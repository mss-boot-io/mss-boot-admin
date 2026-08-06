export const getUserDisplayName = (
  user?: Pick<API.User, 'name' | 'username'>,
): string => {
  const name = user?.name?.trim();
  return name || user?.username?.trim() || '';
};
