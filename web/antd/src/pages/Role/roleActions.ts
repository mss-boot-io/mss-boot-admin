type RoleLifecycleFlags = {
  default?: boolean;
  root?: boolean;
};

export const getRoleActionDisabledState = (role: RoleLifecycleFlags) => {
  const managed = role.root === true || role.default === true;

  return {
    edit: managed,
    delete: managed,
    authorize: role.root === true,
  };
};
