import { PageContainer } from '@ant-design/pro-components';
import { useIntl, useModel } from '@umijs/max';
import OptionListView from '@/modules/option/OptionListView';
import { hasPermission } from '@/shared/auth/access';
import type { InitialState } from '@/shared/auth/types';
import { PageForbidden } from '@/shared/design-system/PageState';

export default function OptionPage() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const user = initialState?.currentUser;
  if (!hasPermission(user, '/option')) {
    return <PageForbidden message={intl.formatMessage({ id: 'option.forbidden.read' })} />;
  }
  return (
    <PageContainer
      content={intl.formatMessage({ id: 'option.description' })}
      title={intl.formatMessage({ id: 'option.title' })}
    >
      <OptionListView
        canCreate={hasPermission(user, '/option/create')}
        canDelete={hasPermission(user, '/option/delete')}
        canEdit={hasPermission(user, '/option/edit')}
      />
    </PageContainer>
  );
}
