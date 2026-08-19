import LanguageEditor from '@mss-admin-core/modules/language/LanguageEditor';
import { hasPermission } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useIntl, useModel } from '@umijs/max';

export default function CreateLanguagePage() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  if (!hasPermission(initialState?.currentUser, '/language/create')) {
    return <PageForbidden message={intl.formatMessage({ id: 'language.forbidden.write' })} />;
  }
  return (
    <PageContainer title={intl.formatMessage({ id: 'language.create.title' })}>
      <LanguageEditor mode="create" />
    </PageContainer>
  );
}
