import { usePresentationIntl } from '@mss-admin-core/modules/presentation-config/messages';
import PresentationConfigConsole from '@mss-admin-core/modules/presentation-config/PresentationConfigConsole';
import { hasPermission } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useModel } from '@umijs/max';

export default function PresentationConfigPage() {
  const intl = usePresentationIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const user = initialState?.currentUser;
  if (!hasPermission(user, '/presentation-config')) {
    return <PageForbidden message={intl.formatMessage({ id: 'presentation.forbidden' })} />;
  }
  return (
    <PageContainer
      content={intl.formatMessage({ id: 'presentation.description' })}
      title={intl.formatMessage({ id: 'presentation.title' })}
    >
      <PresentationConfigConsole
        canDraft={hasPermission(user, '/presentation-config/draft')}
        canPublish={hasPermission(user, '/presentation-config/publish')}
        canRollback={hasPermission(user, '/presentation-config/rollback')}
      />
    </PageContainer>
  );
}
