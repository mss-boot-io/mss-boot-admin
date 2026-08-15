import { PageContainer } from '@ant-design/pro-components';
import { useIntl, useModel } from '@umijs/max';
import LanguageListView from '@/modules/language/LanguageListView';
import { hasPermission } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageForbidden } from '@/shared/design-system/PageState';

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
