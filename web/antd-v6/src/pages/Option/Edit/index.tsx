import OptionEditor from '@mss-admin-core/modules/option/OptionEditor';
import { hasPermission } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useIntl, useModel, useParams } from '@umijs/max';

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
