import { useIntl, useModel } from '@umijs/max';
import OnlineSessionsView from '@/modules/session/OnlineSessionsView';
import { isRootIdentity } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageContainer } from '@/shared/design-system/PageContainer';
import { PageForbidden } from '@/shared/design-system/PageState';

export default function OnlineSessionsPage() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  if (!isRootIdentity(initialState?.currentUser)) {
    return <PageForbidden message={intl.formatMessage({ id: 'sessions.forbidden' })} />;
  }
  return (
    <PageContainer
      title={intl.formatMessage({ id: 'sessions.title' })}
      content={intl.formatMessage({ id: 'sessions.description' })}
    >
      <OnlineSessionsView />
    </PageContainer>
  );
}
