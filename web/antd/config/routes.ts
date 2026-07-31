import routesCustom from './routes.custom';

/**
 * @name umi 的路由配置
 * @description 只支持 path,component,routes,redirect,wrappers,name,icon 的配置
 * @param path  path 只支持两种占位符配置，第一种是动态参数 :id 的形式，第二种是 * 通配符，通配符只能出现路由字符串的最后。
 * @param component 配置 location 和 path 匹配后用于渲染的 React 组件路径。可以是绝对路径，也可以是相对路径，如果是相对路径，会从 src/pages 开始找起。
 * @param routes 配置子路由，通常在需要为多个路径增加 layout 组件时使用。
 * @param redirect 配置路由跳转
 * @param wrappers 配置路由组件的包装组件，通过包装组件可以为当前的路由组件组合进更多的功能。 比如，可以用于路由级别的权限校验
 * @param name 配置路由的标题，默认读取国际化文件 menu.ts 中 menu.xxxx 的值，如配置 name 为 login，则读取 menu.ts 中 menu.login 的取值作为标题
 * @param icon 配置路由的图标，取值参考 https://ant.design/components/icon-cn， 注意去除风格后缀和大小写，如想要配置图标为 <StepBackwardOutlined /> 则取值应为 stepBackward 或 StepBackward，如想要配置图标为 <UserOutlined /> 则取值应为 user 或者 User
 * @doc https://umijs.org/docs/guides/routes
 */
export default [
  {
    path: '/user',
    layout: false,
    routes: [
      {
        path: '/user/login',
        component: './User/Login',
      },
      {
        path: '/user/callback/:provider',
        component: './User/Callback/$provider.tsx',
      },
      {
        path: '/user/forget',
        component: './User/Login/forget.tsx',
      },
      {
        path: '/user/register',
        component: './User/Login/register.tsx',
      },
    ],
  },
  {
    path: '/welcome',
    icon: 'smile',
    component: './Welcome',
  },
  {
    path: '/analysis',
    component: './Welcome',
  },
  {
    path: '/workplace',
    component: './Welcome',
  },
  {
    path: '/generator',
    icon: 'smile',
    component: './Generator',
  },
  {
    path: '/account',
    icon: 'user',
    hideInMenu: true,
    routes: [
      {
        path: '/account/center',
        name: 'center',
        component: './Account/Center',
      },
      {
        path: '/account/settings',
        component: './Account/Settings',
      },
    ],
  },
  {
    icon: 'table',
    path: '/users',
    routes: [
      {
        path: '/users',
        hideInMenu: true,
        component: './User',
      },
      {
        path: '/users/control/:id',
        hideInMenu: true,
        component: './User',
      },
      {
        path: '/users/password-reset/:id',
        hideInMenu: true,
        component: './User/Reset/$id.tsx',
      },
    ],
  },
  {
    icon: 'table',
    path: '/role',
    routes: [
      {
        path: '/role',
        hideInMenu: true,
        component: './Role',
      },
      {
        path: '/role/:id',
        hideInMenu: true,
        component: './Role',
      },
    ],
  },
  {
    icon: 'table',
    path: '/departments',
    routes: [
      {
        path: '/departments',
        hideInMenu: true,
        component: './Department',
      },
      {
        path: '/departments/:id',
        hideInMenu: true,
        component: './Department',
      },
    ],
  },
  {
    icon: 'table',
    path: '/posts',
    routes: [
      {
        path: '/posts',
        hideInMenu: true,
        component: './Post',
      },
      {
        path: '/posts/:id',
        hideInMenu: true,
        component: './Post',
      },
    ],
  },
  {
    icon: 'table',
    path: '/task',
    routes: [
      {
        path: '/task',
        hideInMenu: true,
        component: './Task',
      },
      {
        path: '/task/:id',
        hideInMenu: true,
        component: './Task',
      },
    ],
  },
  {
    path: '/language',
    icon: 'translation',
    routes: [
      {
        path: '/language',
        hideInMenu: true,
        component: './Language',
      },
      {
        path: '/language/:id',
        component: './Language',
      },
    ],
  },
  {
    path: '/menu',
    icon: 'menu',
    routes: [
      {
        path: '/menu',
        hideInMenu: true,
        component: './Menu/index.tsx',
      },
      {
        path: '/menu/:id',
        component: './Menu/index.tsx',
      },
    ],
  },
  {
    path: '/app-config',
    routes: [
      {
        path: '/app-config',
        hideInMenu: true,
        component: './AppConfig',
      },
    ],
  },
  {
    path: '/system-config',
    routes: [
      {
        path: '/system-config',
        hideInMenu: true,
        component: './SystemConfig',
      },
      {
        path: '/system-config/:id',
        component: './SystemConfig',
      },
    ],
  },
  {
    path: '/notice',
    routes: [
      {
        path: '/notice',
        hideInMenu: true,
        component: './Notice',
      },
      // {
      //   path: '/notice/:id',
      //   component: './Notice',
      // },
    ],
  },
  {
    path: '/model',
    routes: [
      {
        path: '/model',
        hideInMenu: true,
        component: './Model',
      },
      {
        path: '/model/:id',
        component: './Model',
      },
    ],
  },
  {
    path: '/field/:modelID',
    routes: [
      {
        path: '/field/:modelID',
        hideInMenu: true,
        component: './Field',
      },
      {
        path: '/field/:modelID/:id',
        component: './Field',
      },
    ],
  },
  {
    path: '/option',
    routes: [
      {
        path: '/option',
        hideInMenu: true,
        component: './Option',
      },
      {
        path: '/option/:id',
        component: './Option',
      },
    ],
  },
  {
    path: '/virtual/:key',
    routes: [
      {
        path: '/virtual/:key',
        hideInMenu: true,
        component: './Virtual',
      },
      {
        path: '/virtual/:key/:id',
        component: './Virtual',
      },
    ],
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
        component: './OnlineSession',
      },
    ],
  },
  {
    path: '/log',
    name: 'system.log',
    icon: 'fileText',
    routes: [
      {
        path: '/log',
        hideInMenu: true,
        component: './Log',
      },
    ],
  },
  {
    path: '/',
    redirect: '/workplace',
  },
  {
    path: '*',
    layout: false,
    component: './404',
  },
  ...routesCustom,
];
