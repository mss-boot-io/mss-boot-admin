import OptionEditor from '@mss-admin-core/modules/option/OptionEditor';
import { hasPermission } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useIntl, useModel } from '@umijs/max';

export default function CreateOptionPage() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  if (!hasPermission(initialState?.currentUser, '/option/create')) {
    return <PageForbidden message={intl.formatMessage({ id: 'option.forbidden.write' })} />;
  }
  return (
    <PageContainer title={intl.formatMessage({ id: 'option.create.title' })}>
      <OptionEditor mode="create" />
    </PageContainer>
  );
}
