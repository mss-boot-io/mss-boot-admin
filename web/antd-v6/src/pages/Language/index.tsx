import LanguageListView from '@mss-admin-core/modules/language/LanguageListView';
import { languagePresentationRegistryEntry } from '@mss-admin-core/modules/language/tablePresentation';
import { hasPermission } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { usePagePresentation } from '@mss-admin-core/shared/presentation/runtime';
import { useIntl, useModel } from '@umijs/max';

function LanguagePresentationPage({
  canCreate,
  canDelete,
  canEdit,
  initialState,
}: {
  canCreate: boolean;
  canDelete: boolean;
  canEdit: boolean;
  initialState: InitialState;
}) {
  const intl = useIntl();
  const presentationRuntime = usePagePresentation(
    languagePresentationRegistryEntry,
    intl.locale === 'en-US' ? 'en-US' : 'zh-CN',
    initialState.currentUser,
    initialState.authorizationVersion,
  );

  return (
    <PageContainer
      content={intl.formatMessage({ id: 'language.description' })}
      title={presentationRuntime.model.title}
    >
      <LanguageListView
        canCreate={canCreate}
        canDelete={canDelete}
        canEdit={canEdit}
        presentationRuntime={presentationRuntime}
      />
    </PageContainer>
  );
}

export default function LanguagePage() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const user = initialState?.currentUser;
  if (!hasPermission(user, '/language')) {
    return <PageForbidden message={intl.formatMessage({ id: 'language.forbidden.read' })} />;
  }
  return (
    <LanguagePresentationPage
      canCreate={hasPermission(user, '/language/create')}
      canDelete={hasPermission(user, '/language/delete')}
      canEdit={hasPermission(user, '/language/edit')}
      initialState={initialState as InitialState}
    />
  );
}
