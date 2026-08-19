import LanguageListView from '@mss-admin-core/modules/language/LanguageListView';
import { hasPermission } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useIntl, useModel } from '@umijs/max';

export default function LanguagePage() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const user = initialState?.currentUser;
  if (!hasPermission(user, '/language')) {
    return <PageForbidden message={intl.formatMessage({ id: 'language.forbidden.read' })} />;
  }
  return (
    <PageContainer
      content={intl.formatMessage({ id: 'language.description' })}
      title={intl.formatMessage({ id: 'language.title' })}
    >
      <LanguageListView
        canCreate={hasPermission(user, '/language/create')}
        canDelete={hasPermission(user, '/language/delete')}
        canEdit={hasPermission(user, '/language/edit')}
      />
    </PageContainer>
  );
}
