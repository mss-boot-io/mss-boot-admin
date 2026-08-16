import { useIntl, useModel, useParams } from '@umijs/max';
import OptionEditor from '@/modules/option/OptionEditor';
import { hasPermission } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageContainer } from '@/shared/design-system/PageContainer';
import { PageForbidden } from '@/shared/design-system/PageState';

export default function EditOptionPage() {
  const intl = useIntl();
  const { id } = useParams<{ id: string }>();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  if (!hasPermission(initialState?.currentUser, '/option/edit')) {
    return <PageForbidden message={intl.formatMessage({ id: 'option.forbidden.write' })} />;
  }
  return (
    <PageContainer title={intl.formatMessage({ id: 'option.edit.title' })}>
      <OptionEditor id={id} mode="edit" />
    </PageContainer>
  );
}
