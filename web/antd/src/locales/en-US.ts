import component from './en-US/component';
import globalHeader from './en-US/globalHeader';
import menu from './en-US/menu';
import pages from './en-US/pages';
import pwa from './en-US/pwa';
import settingDrawer from './en-US/settingDrawer';
import settings from './en-US/settings';
import custom from './en-US/custom';

export default {
  'navBar.lang': 'Languages',
  'layout.user.link.help': 'Help',
  'layout.user.link.privacy': 'Privacy',
  'layout.user.link.terms': 'Terms',
  'app.copyright.produced': 'Produced by the open source organization mss-boot-io',
  'app.preview.down.block': 'Download this page to your local project',
  'app.welcome.link.fetch-blocks': 'Get all block',
  'app.welcome.link.block-list': 'Quickly build standard, pages based on `block` development',
  'app.documentation': 'Documentation',
  ...globalHeader,
  ...menu,
  ...settingDrawer,
  ...settings,
  ...pwa,
  ...component,
  ...pages,
  ...custom,
  'pages.form.required': 'This field is required',
  'pages.form.placeholder': 'Please input',
  'pages.form.select.placeholder': 'Please select',
  'pages.appConfig.base.websiteName': 'Website Name',
  'pages.appConfig.base.websiteDescription': 'Website Description',
  'pages.appConfig.base.websiteLogo': 'Website Logo',
  'pages.appConfig.base.websiteRecordNumber': 'Record Number',
  'pages.appConfig.base.websiteCopyRight': 'Copyright',
};
