import type { ProLayoutProps } from '@ant-design/pro-components';

export const defaultSettings = {
  navTheme: 'light',
  colorPrimary: '#1677ff',
  layout: 'mix',
  contentWidth: 'Fluid',
  fixedHeader: false,
  fixSiderbar: true,
  colorWeak: false,
  title: 'mss-boot-io',
  logo: '/logo.svg',
  splitMenus: false,
} satisfies ProLayoutProps & { logo: string };

export default defaultSettings;
