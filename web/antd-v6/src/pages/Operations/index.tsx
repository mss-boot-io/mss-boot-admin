import { useIntl, useLocation, useModel } from '@umijs/max';
import LogViewer from '@/modules/operations/LogViewer';
import NoticeCenter from '@/modules/operations/NoticeCenter';
import SystemConfigManagement from '@/modules/operations/SystemConfigManagement';
import TaskManagement from '@/modules/operations/TaskManagement';
import { hasPermission, isRootIdentity } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageContainer } from '@/shared/design-system/PageContainer';
import { PageForbidden } from '@/shared/design-system/PageState';
import type { ManagementRouteIntent } from '@/shared/navigation/managementRoute';

interface OperationsRouteDefinition {
  description: string;
  permission: string;
  rootOnly?: boolean;
  render: (
    user: NonNullable<InitialState['currentUser']>,
    root: boolean,
    intent?: ManagementRouteIntent,
  ) => React.ReactNode;
  title: string;
}

const operationsRoutes: Record<string, OperationsRouteDefinition> = {
  '/task': {
    permission: '/task',
    title: 'task.title',
    description: 'task.description',
    render: (_user, root, routeIntent) => <TaskManagement root={root} routeIntent={routeIntent} />,
  },
  '/notice': {
    permission: '/notice',
    title: 'notice.title',
    description: 'notice.description',
    render: (user) => <NoticeCenter canMarkRead={hasPermission(user, '/notice/read')} />,
  },
  '/log': {
    permission: '/log',
    title: 'log.title',
    description: 'log.description',
    render: (user) => (
      <LogViewer
        canExportRuntime={hasPermission(user, '/log/export')}
        canReadRuntime={hasPermission(user, '/log/runtime')}
      />
    ),
  },
  '/system-config': {
    permission: '/system-config',
    rootOnly: true,
    title: 'systemConfig.title',
    description: 'systemConfig.description',
    render: (_user, _root, routeIntent) => <SystemConfigManagement routeIntent={routeIntent} />,
  },
};

function routeIntent(pathname: string, basePath: string): ManagementRouteIntent | undefined {
  if (pathname === basePath) return undefined;
  if (pathname === `${basePath}/create`) return { action: 'create' };
  const id = pathname.match(new RegExp(`^${basePath}/([^/]+)$`))?.[1];
  return id ? { action: 'edit', id: decodeURIComponent(id) } : undefined;
}

export default function OperationsPage() {
  const intl = useIntl();
  const location = useLocation();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const rawPathname = location.pathname.replace(/\/+$/, '') || '/';
  const pathname = Object.keys(operationsRoutes).find(
    (candidate) => rawPathname === candidate || rawPathname.startsWith(`${candidate}/`),
  );
  const route = pathname ? operationsRoutes[pathname] : undefined;
  const user = initialState?.currentUser;
  const root = isRootIdentity(user);

  if (!route || !user || !hasPermission(user, route.permission) || (route.rootOnly && !root)) {
    return <PageForbidden message={intl.formatMessage({ id: 'operations.forbidden.read' })} />;
  }

  const intent = routeIntent(rawPathname, pathname ?? rawPathname);

  return (
    <PageContainer
      content={intl.formatMessage({ id: route.description })}
      title={intl.formatMessage({ id: route.title })}
    >
      {route.render(user, root, intent)}
    </PageContainer>
  );
}
