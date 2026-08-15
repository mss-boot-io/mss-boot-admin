import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Tabs } from 'antd';
import ThemeSettingsEditor from '@/modules/theme/ThemeSettingsEditor';

export default function AppConfigPage() {
  const intl = useIntl();
  return (
    <PageContainer title={intl.formatMessage({ id: 'pages.appConfig.title' })}>
      <Tabs
        items={[
          {
            key: 'theme',
            label: intl.formatMessage({ id: 'pages.theme.title' }),
            children: <ThemeSettingsEditor scope="application" />,
          },
        ]}
      />
    </PageContainer>
  );
}
