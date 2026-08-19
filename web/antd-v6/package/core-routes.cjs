const { existsSync } = require('node:fs');
const { resolve } = require('node:path');

const coreRoutes = [
  {
    path: '/user',
    layout: false,
    routes: [
      {
        path: '/user/login',
        name: 'login',
        component: './User/Login',
      },
      {
        path: '/user/register',
        name: 'register',
        component: './User/Register',
      },
      {
        path: '/user/forget',
        name: 'forget-password',
        component: './User/Forget',
      },
      {
        path: '/user/oauth/callback/:provider',
        name: 'oauth-callback',
        component: './User/OAuthCallback',
      },
      {
        path: '/user/*',
        component: './Exception/404',
      },
    ],
  },
  {
    path: '/workplace',
    name: 'workplace',
    icon: 'dashboard',
    component: './Workplace',
    access: 'canAccessRoute',
    permission: '/welcome',
  },
  {
    path: '/welcome',
    redirect: '/workplace',
  },
  {
    path: '/analysis',
    redirect: '/workplace',
  },
  {
    path: '/users',
    name: 'users',
    icon: 'user',
    component: './Administration',
    access: 'canAccessRoute',
    permission: '/users',
  },
  {
    path: '/users/control/create',
    hideInMenu: true,
    component: './Administration',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/users/control/:id',
    hideInMenu: true,
    component: './Administration',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/users/password-reset/:id',
    hideInMenu: true,
    component: './Administration',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/role',
    name: 'role',
    icon: 'team',
    component: './Administration',
    access: 'canAccessRoute',
    permission: '/role',
  },
  {
    path: '/role/create',
    hideInMenu: true,
    component: './Administration',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/role/:id',
    hideInMenu: true,
    component: './Administration',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/menu',
    name: 'menu-management',
    icon: 'menu',
    component: './Administration',
    access: 'canAccessRoute',
    permission: '/menu',
  },
  {
    path: '/menu/create',
    hideInMenu: true,
    component: './Administration',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/menu/:id',
    hideInMenu: true,
    component: './Administration',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/departments',
    name: 'departments',
    icon: 'apartment',
    component: './Administration',
    access: 'canAccessRoute',
    permission: '/departments',
  },
  {
    path: '/departments/create',
    hideInMenu: true,
    component: './Administration',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/departments/:id',
    hideInMenu: true,
    component: './Administration',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/posts',
    name: 'posts',
    icon: 'cluster',
    component: './Administration',
    access: 'canAccessRoute',
    permission: '/posts',
  },
  {
    path: '/posts/create',
    hideInMenu: true,
    component: './Administration',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/posts/:id',
    hideInMenu: true,
    component: './Administration',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/task',
    name: 'task',
    icon: 'wallet',
    component: './Operations',
    access: 'canAccessRoute',
    permission: '/task',
  },
  {
    path: '/task/create',
    hideInMenu: true,
    component: './Operations',
    access: 'canAccessRoute',
    permission: '/task/create',
  },
  {
    path: '/task/:id',
    hideInMenu: true,
    component: './Operations',
    access: 'canAccessRoute',
    permission: '/task/edit',
  },
  {
    path: '/notice',
    name: 'notice',
    icon: 'message',
    component: './Operations',
    access: 'canAccessRoute',
    permission: '/notice',
  },
  {
    path: '/log',
    name: 'system-log',
    icon: 'fileText',
    component: './Operations',
    access: 'canAccessRoute',
    permission: '/log',
  },
  {
    path: '/system-config',
    name: 'system-config',
    icon: 'inbox',
    component: './Operations',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/system-config/create',
    hideInMenu: true,
    component: './Operations',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/system-config/:id',
    hideInMenu: true,
    component: './Operations',
    access: 'canAccessRoute',
    rootOnly: true,
  },
  {
    path: '/app-config',
    name: 'app-config',
    icon: 'setting',
    component: './AppConfig',
    access: 'canAccessRoute',
    permission: '/app-config',
  },
  {
    path: '/language',
    name: 'language',
    icon: 'translation',
    component: './Language',
    access: 'canAccessRoute',
    permission: '/language',
  },
  {
    path: '/language/create',
    hideInMenu: true,
    component: './Language/Create',
    access: 'canAccessRoute',
    permission: '/language/create',
  },
  {
    path: '/language/:id',
    hideInMenu: true,
    component: './Language/Edit',
    access: 'canAccessRoute',
    permission: '/language/edit',
  },
  {
    path: '/option',
    name: 'option',
    icon: 'unorderedList',
    component: './Option',
    access: 'canAccessRoute',
    permission: '/option',
  },
  {
    path: '/option/create',
    hideInMenu: true,
    component: './Option/Create',
    access: 'canAccessRoute',
    permission: '/option/create',
  },
  {
    path: '/option/:id',
    hideInMenu: true,
    component: './Option/Edit',
    access: 'canAccessRoute',
    permission: '/option/edit',
  },
  {
    path: '/security',
    name: 'security',
    icon: 'safety',
    routes: [
      {
        path: '/security',
        redirect: '/security/online-sessions',
      },
      {
        path: '/security/online-sessions',
        name: 'online-sessions',
        icon: 'desktop',
        component: './Security/OnlineSessions',
        access: 'canAccessRoute',
        rootOnly: true,
      },
    ],
  },
  {
    path: '/account',
    hideInMenu: true,
    routes: [
      {
        path: '/account/center',
        name: 'account-center',
        component: './Account/Center',
        access: 'canAccessRoute',
      },
      {
        path: '/account/settings',
        name: 'account-settings',
        component: './Account/Settings',
        access: 'canAccessRoute',
      },
    ],
  },
];

const fallbackRoutes = [
  {
    path: '/403',
    layout: false,
    component: './Exception/403',
  },
  {
    path: '/',
    redirect: '/workplace',
  },
  {
    path: '/*',
    layout: false,
    component: './Exception/404',
  },
];

function routePaths(routes) {
  return routes
    .flatMap((route) => [
      typeof route.path === 'string' ? route.path : undefined,
      ...routePaths(Array.isArray(route.routes) ? route.routes : []),
    ])
    .filter(Boolean);
}

function resolveCoreComponent(pagesRoot, component) {
  const base = resolve(pagesRoot, component.slice(2));
  const candidates = [
    `${base}.tsx`,
    `${base}.ts`,
    resolve(base, 'index.tsx'),
    resolve(base, 'index.ts'),
  ];
  const resolved = candidates.find((candidate) => existsSync(candidate));
  if (!resolved) {
    throw new Error(`Admin route component does not exist: ${component}`);
  }
  return resolved;
}

function qualifyCoreComponents(routes, pagesRoot) {
  return routes.map((route) => ({
    ...route,
    ...(typeof route.component === 'string' && route.component.startsWith('./')
      ? { component: resolveCoreComponent(pagesRoot, route.component) }
      : {}),
    ...(Array.isArray(route.routes)
      ? { routes: qualifyCoreComponents(route.routes, pagesRoot) }
      : {}),
  }));
}

function createAdminRoutes(options = {}) {
  const businessRoutes = Array.isArray(options.businessRoutes) ? options.businessRoutes : [];
  const pagesRoot = options.pagesRoot || resolve(__dirname, '../src/pages');
  const reserved = new Set(routePaths([...coreRoutes, ...fallbackRoutes]));
  const seen = new Set();
  for (const routePath of routePaths(businessRoutes)) {
    if (reserved.has(routePath)) {
      throw new Error(`business route conflicts with an Admin route: ${routePath}`);
    }
    if (seen.has(routePath)) {
      throw new Error(`duplicate business route: ${routePath}`);
    }
    seen.add(routePath);
  }
  return [
    ...qualifyCoreComponents(coreRoutes, pagesRoot),
    ...businessRoutes,
    ...qualifyCoreComponents(fallbackRoutes, pagesRoot),
  ];
}

module.exports = {
  coreRoutes,
  createAdminRoutes,
  fallbackRoutes,
  routePaths,
};
