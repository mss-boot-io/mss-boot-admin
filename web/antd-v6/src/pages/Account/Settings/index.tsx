import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import ThemeSettingsEditor from '@/modules/theme/ThemeSettingsEditor';

export default function AccountSettingsPage() {
  const intl = useIntl();
  return (
    <PageContainer title={intl.formatMessage({ id: 'pages.accountSettings.title' })}>
      <ThemeSettingsEditor scope="user" />
    </PageContainer>
  );
}
