import { PageContainer } from '@ant-design/pro-components';
import { useIntl, useLocation, useModel } from '@umijs/max';
import LogViewer from '@/modules/operations/LogViewer';
import NoticeCenter from '@/modules/operations/NoticeCenter';
import SystemConfigManagement from '@/modules/operations/SystemConfigManagement';
import TaskManagement from '@/modules/operations/TaskManagement';
import { hasPermission, isRootIdentity } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageForbidden } from '@/shared/design-system/PageState';

interface OperationsRouteDefinition {
  description: string;
  permission: string;
  rootOnly?: boolean;
  render: (user: NonNullable<InitialState['currentUser']>, root: boolean) => React.ReactNode;
  title: string;
}

const operationsRoutes: Record<string, OperationsRouteDefinition> = {
  '/task': {
    permission: '/task',
    title: 'task.title',
    description: 'task.description',
    render: (_user, root) => <TaskManagement root={root} />,
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
    render: () => <SystemConfigManagement />,
  },
};

export default function OperationsPage() {
  const intl = useIntl();
  const location = useLocation();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const pathname = location.pathname.replace(/\/+$/, '') || '/';
  const route = operationsRoutes[pathname];
  const user = initialState?.currentUser;
  const root = isRootIdentity(user);

  if (!route || !user || !hasPermission(user, route.permission) || (route.rootOnly && !root)) {
    return <PageForbidden message={intl.formatMessage({ id: 'operations.forbidden.read' })} />;
  }

  return (
    <PageContainer
      content={intl.formatMessage({ id: route.description })}
      title={intl.formatMessage({ id: route.title })}
    >
      {route.render(user, root)}
    </PageContainer>
  );
}
