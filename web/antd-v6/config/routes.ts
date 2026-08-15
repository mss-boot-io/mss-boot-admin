import generatedRoutes from './routes.generated';

const routes = [
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
        path: '/user/callback/:provider',
        name: 'oauth-callback',
        component: './User/OAuthCallback',
      },
      {
        path: '/user/oauth/callback/:provider',
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
    path: '/migration',
    name: 'migration',
    icon: 'deploymentUnit',
    component: './Migration',
    access: 'canAccessRoute',
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
  ...generatedRoutes,
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

export default routes;
