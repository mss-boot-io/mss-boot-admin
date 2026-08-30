import OnlineSessionsView from '@mss-admin-core/modules/session/OnlineSessionsView';
import { onlineSessionPresentationRegistryEntry } from '@mss-admin-core/modules/session/tablePresentation';
import { isRootIdentity } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { usePagePresentation } from '@mss-admin-core/shared/presentation/runtime';
import { useIntl, useModel } from '@umijs/max';

function OnlineSessionsPresentationPage({ initialState }: { initialState: InitialState }) {
  const intl = useIntl();
  const presentationRuntime = usePagePresentation(
    onlineSessionPresentationRegistryEntry,
    intl.locale === 'en-US' ? 'en-US' : 'zh-CN',
    initialState.currentUser,
    initialState.authorizationVersion,
  );

  return (
    <PageContainer
      title={presentationRuntime.model.title}
      content={intl.formatMessage({ id: 'sessions.description' })}
    >
      <OnlineSessionsView presentationRuntime={presentationRuntime} />
    </PageContainer>
  );
}

export default function OnlineSessionsPage() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  if (!isRootIdentity(initialState?.currentUser)) {
    return <PageForbidden message={intl.formatMessage({ id: 'sessions.forbidden' })} />;
  }
  return <OnlineSessionsPresentationPage initialState={initialState as InitialState} />;
}
