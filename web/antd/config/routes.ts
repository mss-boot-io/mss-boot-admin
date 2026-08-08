import routesCustom from './routes.custom';
import { PUBLIC_ROUTE_PATHS } from '../src/utils/routeAccess';

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
        path: PUBLIC_ROUTE_PATHS.login,
        component: './User/Login',
      },
      {
        path: PUBLIC_ROUTE_PATHS.callback,
        component: './User/Callback/$provider.tsx',
      },
      {
        path: PUBLIC_ROUTE_PATHS.forget,
        component: './User/Login/forget.tsx',
      },
      {
        path: PUBLIC_ROUTE_PATHS.register,
        component: './User/Login/register.tsx',
      },
    ],
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
    path: '/workplace',
    icon: 'smile',
    component: './Welcome',
    access: 'canAccessRoute',
    permission: '/welcome',
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
    access: 'canAccessRoute',
    permission: '/users',
    routes: [
      {
        path: '/users',
        hideInMenu: true,
        component: './User',
        access: 'canAccessRoute',
        permission: '/users',
      },
      {
        path: '/users/control/create',
        hideInMenu: true,
        component: './User',
        access: 'canAccessRoute',
        rootOnly: true,
      },
      {
        path: '/users/control/:id',
        hideInMenu: true,
        component: './User',
        access: 'canAccessRoute',
        rootOnly: true,
      },
      {
        path: '/users/password-reset/:id',
        hideInMenu: true,
        component: './User/Reset/$id.tsx',
        access: 'canAccessRoute',
        permission: '/users/password-reset',
      },
    ],
  },
  {
    icon: 'table',
    path: '/role',
    access: 'canAccessRoute',
    permission: '/role',
    routes: [
      {
        path: '/role',
        hideInMenu: true,
        component: './Role',
        access: 'canAccessRoute',
        permission: '/role',
      },
      {
        path: '/role/create',
        hideInMenu: true,
        component: './Role',
        access: 'canAccessRoute',
        rootOnly: true,
      },
      {
        path: '/role/:id',
        hideInMenu: true,
        component: './Role',
        access: 'canAccessRoute',
        rootOnly: true,
      },
    ],
  },
  {
    icon: 'table',
    path: '/departments',
    access: 'canAccessRoute',
    permission: '/departments',
    routes: [
      {
        path: '/departments',
        hideInMenu: true,
        component: './Department',
        access: 'canAccessRoute',
        permission: '/departments',
      },
      {
        path: '/departments/create',
        hideInMenu: true,
        component: './Department',
        access: 'canAccessRoute',
        rootOnly: true,
      },
      {
        path: '/departments/:id',
        hideInMenu: true,
        component: './Department',
        access: 'canAccessRoute',
        rootOnly: true,
      },
    ],
  },
  {
    icon: 'table',
    path: '/posts',
    access: 'canAccessRoute',
    permission: '/posts',
    routes: [
      {
        path: '/posts',
        hideInMenu: true,
        component: './Post',
        access: 'canAccessRoute',
        permission: '/posts',
      },
      {
        path: '/posts/create',
        hideInMenu: true,
        component: './Post',
        access: 'canAccessRoute',
        rootOnly: true,
      },
      {
        path: '/posts/:id',
        hideInMenu: true,
        component: './Post',
        access: 'canAccessRoute',
        rootOnly: true,
      },
    ],
  },
  {
    icon: 'table',
    path: '/task',
    access: 'canAccessRoute',
    permission: '/task',
    routes: [
      {
        path: '/task',
        hideInMenu: true,
        component: './Task',
        access: 'canAccessRoute',
        permission: '/task',
      },
      {
        path: '/task/create',
        hideInMenu: true,
        component: './Task',
        access: 'canAccessRoute',
        permission: '/task/create',
      },
      {
        path: '/task/:id',
        hideInMenu: true,
        component: './Task',
        access: 'canAccessRoute',
        permission: '/task/edit',
      },
    ],
  },
  {
    path: '/language',
    icon: 'translation',
    access: 'canAccessRoute',
    permission: '/language',
    routes: [
      {
        path: '/language',
        hideInMenu: true,
        component: './Language',
        access: 'canAccessRoute',
        permission: '/language',
      },
      {
        path: '/language/create',
        hideInMenu: true,
        component: './Language',
        access: 'canAccessRoute',
        permission: '/language/create',
      },
      {
        path: '/language/:id',
        component: './Language',
        access: 'canAccessRoute',
        permission: '/language/edit',
      },
    ],
  },
  {
    path: '/menu',
    icon: 'menu',
    access: 'canAccessRoute',
    permission: '/menu',
    routes: [
      {
        path: '/menu',
        hideInMenu: true,
        component: './Menu/index.tsx',
        access: 'canAccessRoute',
        permission: '/menu',
      },
      {
        path: '/menu/create',
        hideInMenu: true,
        component: './Menu/index.tsx',
        access: 'canAccessRoute',
        rootOnly: true,
      },
      {
        path: '/menu/:id',
        component: './Menu/index.tsx',
        access: 'canAccessRoute',
        rootOnly: true,
      },
    ],
  },
  {
    path: '/app-config',
    access: 'canAccessRoute',
    permission: '/app-config',
    routes: [
      {
        path: '/app-config',
        hideInMenu: true,
        component: './AppConfig',
        access: 'canAccessRoute',
        permission: '/app-config',
      },
    ],
  },
  {
    path: '/system-config',
    access: 'canAccessRoute',
    rootOnly: true,
    routes: [
      {
        path: '/system-config',
        hideInMenu: true,
        component: './SystemConfig',
        access: 'canAccessRoute',
        rootOnly: true,
      },
      {
        path: '/system-config/create',
        hideInMenu: true,
        component: './SystemConfig',
        access: 'canAccessRoute',
        rootOnly: true,
      },
      {
        path: '/system-config/:id',
        component: './SystemConfig',
        access: 'canAccessRoute',
        rootOnly: true,
      },
    ],
  },
  {
    path: '/notice',
    access: 'canAccessRoute',
    permission: '/notice',
    routes: [
      {
        path: '/notice',
        hideInMenu: true,
        component: './Notice',
        access: 'canAccessRoute',
        permission: '/notice',
      },
      // {
      //   path: '/notice/:id',
      //   component: './Notice',
      // },
    ],
  },
  {
    path: '/option',
    access: 'canAccessRoute',
    permission: '/option',
    routes: [
      {
        path: '/option',
        hideInMenu: true,
        component: './Option',
        access: 'canAccessRoute',
        permission: '/option',
      },
      {
        path: '/option/create',
        hideInMenu: true,
        component: './Option',
        access: 'canAccessRoute',
        permission: '/option/create',
      },
      {
        path: '/option/:id',
        component: './Option',
        access: 'canAccessRoute',
        permission: '/option/edit',
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
        access: 'canAccessRoute',
        rootOnly: true,
      },
    ],
  },
  {
    path: '/log',
    name: 'system.log',
    icon: 'fileText',
    access: 'canAccessRoute',
    permission: '/log',
    routes: [
      {
        path: '/log',
        hideInMenu: true,
        component: './Log',
        access: 'canAccessRoute',
        permission: '/log',
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
