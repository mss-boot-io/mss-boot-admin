import LanguageEditor from '@mss-admin-core/modules/language/LanguageEditor';
import { hasPermission } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useIntl, useModel, useParams } from '@umijs/max';

export default function EditLanguagePage() {
  const intl = useIntl();
  const { id } = useParams<{ id: string }>();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  if (!hasPermission(initialState?.currentUser, '/language/edit')) {
    return <PageForbidden message={intl.formatMessage({ id: 'language.forbidden.write' })} />;
  }
  return (
    <PageContainer title={intl.formatMessage({ id: 'language.edit.title' })}>
      <LanguageEditor id={id} mode="edit" />
    </PageContainer>
  );
}
