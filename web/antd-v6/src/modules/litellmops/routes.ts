export default [
  {
    path: '/litellm-ops/gateway',
    serverPaths: ['/litellm-ops/gateway'],
    menuName: 'gateway',
    permission: 'litellmops:gateway-read',
  },
  {
    path: '/litellm-ops/users',
    serverPaths: ['/litellm-ops/users'],
    menuName: 'users',
    permission: 'litellmops:user-list',
  },
  {
    path: '/litellm-ops/keys',
    serverPaths: ['/litellm-ops/keys'],
    menuName: 'keys',
    permission: 'litellmops:key-list',
  },
  {
    path: '/litellm-ops/bills',
    serverPaths: ['/litellm-ops/bills'],
    menuName: 'bills',
    permission: 'litellmops:bills',
  },
  {
    path: '/litellm-ops/orgs',
    serverPaths: ['/litellm-ops/orgs'],
    menuName: 'orgs',
    permission: 'litellmops:org-list',
  },
  {
    path: '/litellm-ops/sales',
    serverPaths: ['/litellm-ops/sales'],
    menuName: 'sales',
    permission: 'litellmops:order-read',
  },
] as const;
