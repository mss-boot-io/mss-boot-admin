import { useIntl, useLocation, useModel } from '@umijs/max';
import DepartmentManagement from '@/modules/administration/DepartmentManagement';
import MenuManagement from '@/modules/administration/MenuManagement';
import PostManagement from '@/modules/administration/PostManagement';
import RoleManagement from '@/modules/administration/RoleManagement';
import UserManagement from '@/modules/administration/UserManagement';
import { hasPermission, isRootIdentity } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageContainer } from '@/shared/design-system/PageContainer';
import { PageForbidden } from '@/shared/design-system/PageState';
import type { ManagementRouteIntent } from '@/shared/navigation/managementRoute';

interface AdministrationRouteDefinition {
  description: string;
  permission: string;
  render: (root: boolean, intent?: ManagementRouteIntent) => React.ReactNode;
  title: string;
}

const administrationRoutes: Record<string, AdministrationRouteDefinition> = {
  '/users': {
    permission: '/users',
    title: 'user.title',
    description: 'user.description',
    render: (root, routeIntent) => (
      <UserManagement
        canCreate={root}
        canDelete={root}
        canEdit={root}
        canResetPassword={root}
        routeIntent={routeIntent}
      />
    ),
  },
  '/role': {
    permission: '/role',
    title: 'role.title',
    description: 'role.description',
    render: (root, routeIntent) => (
      <RoleManagement
        canAuthorize={root}
        canCreate={root}
        canDelete={root}
        canEdit={root}
        routeIntent={routeIntent}
      />
    ),
  },
  '/menu': {
    permission: '/menu',
    title: 'menu.title',
    description: 'menu.description',
    render: (root, routeIntent) => (
      <MenuManagement
        canBindAPI={root}
        canCreate={root}
        canDelete={root}
        canEdit={root}
        routeIntent={routeIntent}
      />
    ),
  },
  '/departments': {
    permission: '/departments',
    title: 'department.title',
    description: 'department.description',
    render: (root, routeIntent) => (
      <DepartmentManagement
        canCreate={root}
        canDelete={root}
        canEdit={root}
        routeIntent={routeIntent}
      />
    ),
  },
  '/posts': {
    permission: '/posts',
    title: 'post.title',
    description: 'post.description',
    render: (root, routeIntent) => (
      <PostManagement canCreate={root} canDelete={root} canEdit={root} routeIntent={routeIntent} />
    ),
  },
};

function routeIntent(pathname: string, basePath: string): ManagementRouteIntent | undefined {
  if (pathname === basePath) return undefined;
  if (pathname === `${basePath}/create` || pathname === '/users/control/create') {
    return { action: 'create' };
  }
  const resetMatch =
    basePath === '/users' ? pathname.match(/^\/users\/password-reset\/([^/]+)$/) : undefined;
  const resetID = resetMatch?.[1];
  if (resetID) return { action: 'reset-password', id: decodeURIComponent(resetID) };
  const editMatch =
    basePath === '/users'
      ? pathname.match(/^\/users\/control\/([^/]+)$/)
      : pathname.match(new RegExp(`^${basePath}/([^/]+)$`));
  const editID = editMatch?.[1];
  return editID ? { action: 'edit', id: decodeURIComponent(editID) } : undefined;
}

export default function AdministrationPage() {
  const intl = useIntl();
  const location = useLocation();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const rawPathname = location.pathname.replace(/\/+$/, '') || '/';
  const pathname = Object.keys(administrationRoutes).find(
    (candidate) => rawPathname === candidate || rawPathname.startsWith(`${candidate}/`),
  );
  const route = pathname ? administrationRoutes[pathname] : undefined;
  const user = initialState?.currentUser;

  if (!route || !hasPermission(user, route.permission)) {
    return <PageForbidden message={intl.formatMessage({ id: 'administration.forbidden.read' })} />;
  }

  const root = isRootIdentity(user);
  const intent = routeIntent(rawPathname, pathname ?? rawPathname);
  return (
    <PageContainer
      content={intl.formatMessage({ id: route.description })}
      title={intl.formatMessage({ id: route.title })}
    >
      {route.render(root, intent)}
    </PageContainer>
  );
}
