import { PageContainer } from '@ant-design/pro-components';
import { useIntl, useLocation, useModel } from '@umijs/max';
import DepartmentManagement from '@/modules/administration/DepartmentManagement';
import MenuManagement from '@/modules/administration/MenuManagement';
import PostManagement from '@/modules/administration/PostManagement';
import RoleManagement from '@/modules/administration/RoleManagement';
import UserManagement from '@/modules/administration/UserManagement';
import { hasPermission, isRootIdentity } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageForbidden } from '@/shared/design-system/PageState';

interface AdministrationRouteDefinition {
  description: string;
  permission: string;
  render: (root: boolean) => React.ReactNode;
  title: string;
}

const administrationRoutes: Record<string, AdministrationRouteDefinition> = {
  '/users': {
    permission: '/users',
    title: 'user.title',
    description: 'user.description',
    render: (root) => (
      <UserManagement canCreate={root} canDelete={root} canEdit={root} canResetPassword={root} />
    ),
  },
  '/role': {
    permission: '/role',
    title: 'role.title',
    description: 'role.description',
    render: (root) => (
      <RoleManagement canAuthorize={root} canCreate={root} canDelete={root} canEdit={root} />
    ),
  },
  '/menu': {
    permission: '/menu',
    title: 'menu.title',
    description: 'menu.description',
    render: (root) => <MenuManagement canCreate={root} canDelete={root} canEdit={root} />,
  },
  '/departments': {
    permission: '/departments',
    title: 'department.title',
    description: 'department.description',
    render: (root) => <DepartmentManagement canCreate={root} canDelete={root} canEdit={root} />,
  },
  '/posts': {
    permission: '/posts',
    title: 'post.title',
    description: 'post.description',
    render: (root) => <PostManagement canCreate={root} canDelete={root} canEdit={root} />,
  },
};

export default function AdministrationPage() {
  const intl = useIntl();
  const location = useLocation();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const pathname = location.pathname.replace(/\/+$/, '') || '/';
  const route = administrationRoutes[pathname];
  const user = initialState?.currentUser;

  if (!route || !hasPermission(user, route.permission)) {
    return <PageForbidden message={intl.formatMessage({ id: 'administration.forbidden.read' })} />;
  }

  const root = isRootIdentity(user);
  return (
    <PageContainer
      content={intl.formatMessage({ id: route.description })}
      title={intl.formatMessage({ id: route.title })}
    >
      {route.render(root)}
    </PageContainer>
  );
}
