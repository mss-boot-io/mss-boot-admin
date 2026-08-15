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
