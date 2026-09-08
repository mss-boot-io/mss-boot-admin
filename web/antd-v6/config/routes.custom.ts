export default [
  {
    path: '/litellm-ops',
    name: 'litellmops',
    icon: 'wallet',
    access: 'canAccessRoute',
    routes: [
      {
        path: '/litellm-ops/users',
        name: 'users',
        icon: 'team',
        component: './LitellmOps/Users',
        access: 'canAccessRoute',
        permission: '/litellmops/users',
      },
      {
        path: '/litellm-ops/keys',
        name: 'keys',
        icon: 'idcard',
        component: './LitellmOps/Keys',
        access: 'canAccessRoute',
        permission: '/litellmops/keys',
      },
      {
        path: '/litellm-ops/bills',
        name: 'bills',
        icon: 'fileText',
        component: './LitellmOps/Bills',
        access: 'canAccessRoute',
        permission: '/litellmops/bills',
      },
    ],
  },
];
