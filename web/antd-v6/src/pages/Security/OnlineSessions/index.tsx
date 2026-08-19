import OnlineSessionsView from '@mss-admin-core/modules/session/OnlineSessionsView';
import { isRootIdentity } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useIntl, useModel } from '@umijs/max';

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
