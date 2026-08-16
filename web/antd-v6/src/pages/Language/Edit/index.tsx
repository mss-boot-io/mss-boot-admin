import { PageContainer } from '@ant-design/pro-components';
import { useIntl, useModel, useParams } from '@umijs/max';
import LanguageEditor from '@/modules/language/LanguageEditor';
import { hasPermission } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageForbidden } from '@/shared/design-system/PageState';

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
