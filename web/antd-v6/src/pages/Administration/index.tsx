import DepartmentManagement from '@mss-admin-core/modules/administration/DepartmentManagement';
import MenuManagement from '@mss-admin-core/modules/administration/MenuManagement';
import PostManagement from '@mss-admin-core/modules/administration/PostManagement';
import RoleManagement from '@mss-admin-core/modules/administration/RoleManagement';
import {
  departmentPresentationRegistryEntry,
  menuPresentationRegistryEntry,
  postPresentationRegistryEntry,
  rolePresentationRegistryEntry,
} from '@mss-admin-core/modules/administration/tablePresentation';
import UserManagement from '@mss-admin-core/modules/administration/UserManagement';
import { userPresentationRegistryEntry } from '@mss-admin-core/modules/administration/userPresentation';
import { hasPermission, isRootIdentity } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import type { ManagementRouteIntent } from '@mss-admin-core/shared/navigation/managementRoute';
import {
  type PagePresentationRuntime,
  type PresentationRegistryEntry,
  usePagePresentation,
} from '@mss-admin-core/shared/presentation/runtime';
import { useIntl, useLocation, useModel } from '@umijs/max';

interface AdministrationRouteDefinition {
  description: string;
  permission: string;
  presentationEntry: PresentationRegistryEntry;
  render: (
    root: boolean,
    intent: ManagementRouteIntent | undefined,
    presentationRuntime: PagePresentationRuntime,
  ) => React.ReactNode;
}

function AdministrationPresentationPage({
  initialState,
  root,
  route,
  routeIntent,
}: {
  initialState?: InitialState;
  root: boolean;
  route: AdministrationRouteDefinition;
  routeIntent?: ManagementRouteIntent;
}) {
  const intl = useIntl();
  const presentationRuntime = usePagePresentation(
    route.presentationEntry,
    intl.locale === 'en-US' ? 'en-US' : 'zh-CN',
    initialState?.currentUser,
    initialState?.authorizationVersion,
  );

  return (
    <PageContainer
      content={intl.formatMessage({ id: route.description })}
      title={presentationRuntime.model.title}
    >
      {route.render(root, routeIntent, presentationRuntime)}
    </PageContainer>
  );
}

const administrationRoutes: Record<string, AdministrationRouteDefinition> = {
  '/users': {
    permission: '/users',
    description: 'user.description',
    presentationEntry: userPresentationRegistryEntry,
    render: (root, routeIntent, presentationRuntime) => (
      <UserManagement
        canCreate={root}
        canDelete={root}
        canEdit={root}
        canResetPassword={root}
        presentationRuntime={presentationRuntime}
        routeIntent={routeIntent}
      />
    ),
  },
  '/role': {
    permission: '/role',
    description: 'role.description',
    presentationEntry: rolePresentationRegistryEntry,
    render: (root, routeIntent, presentationRuntime) => (
      <RoleManagement
        canAuthorize={root}
        canCreate={root}
        canDelete={root}
        canEdit={root}
        presentationRuntime={presentationRuntime}
        routeIntent={routeIntent}
      />
    ),
  },
  '/menu': {
    permission: '/menu',
    description: 'menu.description',
    presentationEntry: menuPresentationRegistryEntry,
    render: (root, routeIntent, presentationRuntime) => (
      <MenuManagement
        canBindAPI={root}
        canCreate={root}
        canDelete={root}
        canEdit={root}
        presentationRuntime={presentationRuntime}
        routeIntent={routeIntent}
      />
    ),
  },
  '/departments': {
    permission: '/departments',
    description: 'department.description',
    presentationEntry: departmentPresentationRegistryEntry,
    render: (root, routeIntent, presentationRuntime) => (
      <DepartmentManagement
        canCreate={root}
        canDelete={root}
        canEdit={root}
        presentationRuntime={presentationRuntime}
        routeIntent={routeIntent}
      />
    ),
  },
  '/posts': {
    permission: '/posts',
    description: 'post.description',
    presentationEntry: postPresentationRegistryEntry,
    render: (root, routeIntent, presentationRuntime) => (
      <PostManagement
        canCreate={root}
        canDelete={root}
        canEdit={root}
        presentationRuntime={presentationRuntime}
        routeIntent={routeIntent}
      />
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
    <AdministrationPresentationPage
      initialState={initialState}
      root={root}
      route={route}
      routeIntent={intent}
    />
  );
}
