import LogViewer from '@mss-admin-core/modules/operations/LogViewer';
import NoticeCenter from '@mss-admin-core/modules/operations/NoticeCenter';
import SystemConfigManagement from '@mss-admin-core/modules/operations/SystemConfigManagement';
import TaskManagement from '@mss-admin-core/modules/operations/TaskManagement';
import {
  auditLogPresentationRegistryEntry,
  loginLogPresentationRegistryEntry,
  noticePresentationRegistryEntry,
  runtimeLogPresentationRegistryEntry,
  systemConfigPresentationRegistryEntry,
  taskPresentationRegistryEntry,
} from '@mss-admin-core/modules/operations/tablePresentation';
import { hasPermission, isRootIdentity } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import type { ManagementRouteIntent } from '@mss-admin-core/shared/navigation/managementRoute';
import { usePagePresentation } from '@mss-admin-core/shared/presentation/runtime';
import { useIntl, useLocation, useModel } from '@umijs/max';

interface OperationsRouteDefinition {
  permission: string;
  rootOnly?: boolean;
  render: (
    initialState: InitialState,
    root: boolean,
    intent?: ManagementRouteIntent,
  ) => React.ReactNode;
}

interface PresentedRouteProps {
  initialState: InitialState;
  root: boolean;
  routeIntent?: ManagementRouteIntent;
}

function runtimeLocale(locale: string) {
  return locale === 'en-US' ? 'en-US' : 'zh-CN';
}

function TaskOperationsPage({ initialState, root, routeIntent }: PresentedRouteProps) {
  const intl = useIntl();
  const presentationRuntime = usePagePresentation(
    taskPresentationRegistryEntry,
    runtimeLocale(intl.locale),
    initialState.currentUser,
    initialState.authorizationVersion,
  );
  return (
    <PageContainer
      content={intl.formatMessage({ id: 'task.description' })}
      title={presentationRuntime.model.title}
    >
      <TaskManagement
        presentationRuntime={presentationRuntime}
        root={root}
        routeIntent={routeIntent}
      />
    </PageContainer>
  );
}

function NoticeOperationsPage({ initialState }: PresentedRouteProps) {
  const intl = useIntl();
  const presentationRuntime = usePagePresentation(
    noticePresentationRegistryEntry,
    runtimeLocale(intl.locale),
    initialState.currentUser,
    initialState.authorizationVersion,
  );
  return (
    <PageContainer
      content={intl.formatMessage({ id: 'notice.description' })}
      title={presentationRuntime.model.title}
    >
      <NoticeCenter
        canMarkRead={hasPermission(initialState.currentUser, '/notice/read')}
        presentationRuntime={presentationRuntime}
      />
    </PageContainer>
  );
}

function SystemConfigOperationsPage({ initialState, routeIntent }: PresentedRouteProps) {
  const intl = useIntl();
  const presentationRuntime = usePagePresentation(
    systemConfigPresentationRegistryEntry,
    runtimeLocale(intl.locale),
    initialState.currentUser,
    initialState.authorizationVersion,
  );
  return (
    <PageContainer
      content={intl.formatMessage({ id: 'systemConfig.description' })}
      title={presentationRuntime.model.title}
    >
      <SystemConfigManagement presentationRuntime={presentationRuntime} routeIntent={routeIntent} />
    </PageContainer>
  );
}

function LogOperationsPage({ initialState }: PresentedRouteProps) {
  const intl = useIntl();
  const locale = runtimeLocale(intl.locale);
  const user = initialState.currentUser;
  const authorizationVersion = initialState.authorizationVersion;
  const loginPresentationRuntime = usePagePresentation(
    loginLogPresentationRegistryEntry,
    locale,
    user,
    authorizationVersion,
  );
  const auditPresentationRuntime = usePagePresentation(
    auditLogPresentationRegistryEntry,
    locale,
    user,
    authorizationVersion,
  );
  const runtimePresentationRuntime = usePagePresentation(
    runtimeLogPresentationRegistryEntry,
    locale,
    user,
    authorizationVersion,
  );
  return (
    <PageContainer
      content={intl.formatMessage({ id: 'log.description' })}
      title={intl.formatMessage({ id: 'log.title' })}
    >
      <LogViewer
        auditPresentationRuntime={auditPresentationRuntime}
        canExportRuntime={hasPermission(user, '/log/export')}
        canReadRuntime={hasPermission(user, '/log/runtime')}
        loginPresentationRuntime={loginPresentationRuntime}
        runtimePresentationRuntime={runtimePresentationRuntime}
      />
    </PageContainer>
  );
}

const operationsRoutes: Record<string, OperationsRouteDefinition> = {
  '/task': {
    permission: '/task',
    render: (initialState, root, routeIntent) => (
      <TaskOperationsPage initialState={initialState} root={root} routeIntent={routeIntent} />
    ),
  },
  '/notice': {
    permission: '/notice',
    render: (initialState, root) => (
      <NoticeOperationsPage initialState={initialState} root={root} />
    ),
  },
  '/log': {
    permission: '/log',
    render: (initialState, root) => <LogOperationsPage initialState={initialState} root={root} />,
  },
  '/system-config': {
    permission: '/system-config',
    rootOnly: true,
    render: (initialState, root, routeIntent) => (
      <SystemConfigOperationsPage
        initialState={initialState}
        root={root}
        routeIntent={routeIntent}
      />
    ),
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

  if (
    !route ||
    !initialState ||
    !user ||
    !hasPermission(user, route.permission) ||
    (route.rootOnly && !root)
  ) {
    return <PageForbidden message={intl.formatMessage({ id: 'operations.forbidden.read' })} />;
  }

  const intent = routeIntent(rawPathname, pathname ?? rawPathname);
  return route.render(initialState, root, intent);
}
