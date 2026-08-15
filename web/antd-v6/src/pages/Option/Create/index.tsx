import { PageContainer } from '@ant-design/pro-components';
import { useIntl, useModel } from '@umijs/max';
import OptionEditor from '@/modules/option/OptionEditor';
import { hasPermission } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageForbidden } from '@/shared/design-system/PageState';

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
