import OptionListView from '@mss-admin-core/modules/option/OptionListView';
import { hasPermission } from '@mss-admin-core/shared/auth/access';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useIntl, useModel } from '@umijs/max';

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
