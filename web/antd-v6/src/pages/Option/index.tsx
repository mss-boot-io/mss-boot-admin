import OptionListView from '@mss-admin-core/modules/option/OptionListView';
import { optionPresentationRegistryEntry } from '@mss-admin-core/modules/option/tablePresentation';
import { hasPermission } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { usePagePresentation } from '@mss-admin-core/shared/presentation/runtime';
import { useIntl, useModel } from '@umijs/max';

function OptionPresentationPage({
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
    optionPresentationRegistryEntry,
    intl.locale === 'en-US' ? 'en-US' : 'zh-CN',
    initialState.currentUser,
    initialState.authorizationVersion,
  );

  return (
    <PageContainer
      content={intl.formatMessage({ id: 'option.description' })}
      title={presentationRuntime.model.title}
    >
      <OptionListView
        canCreate={canCreate}
        canDelete={canDelete}
        canEdit={canEdit}
        presentationRuntime={presentationRuntime}
      />
    </PageContainer>
  );
}

export default function OptionPage() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const user = initialState?.currentUser;
  if (!hasPermission(user, '/option')) {
    return <PageForbidden message={intl.formatMessage({ id: 'option.forbidden.read' })} />;
  }
  return (
    <OptionPresentationPage
      canCreate={hasPermission(user, '/option/create')}
      canDelete={hasPermission(user, '/option/delete')}
      canEdit={hasPermission(user, '/option/edit')}
      initialState={initialState as InitialState}
    />
  );
}
