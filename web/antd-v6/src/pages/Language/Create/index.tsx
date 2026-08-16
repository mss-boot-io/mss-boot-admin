import { useIntl, useModel } from '@umijs/max';
import LanguageEditor from '@/modules/language/LanguageEditor';
import { hasPermission } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageContainer } from '@/shared/design-system/PageContainer';
import { PageForbidden } from '@/shared/design-system/PageState';

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
