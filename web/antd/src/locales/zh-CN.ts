import component from './zh-CN/component';
import globalHeader from './zh-CN/globalHeader';
import menu from './zh-CN/menu';
import pages from './zh-CN/pages';
import pwa from './zh-CN/pwa';
import settingDrawer from './zh-CN/settingDrawer';
import settings from './zh-CN/settings';
import custom from './zh-CN/custom';

export default {
  'navBar.lang': '语言',
  'layout.user.link.help': '帮助',
  'layout.user.link.privacy': '隐私',
  'layout.user.link.terms': '条款',
  'app.copyright.produced': '开源组织mss-boot-io出品',
  'app.preview.down.block': '下载此页面到本地项目',
  'app.welcome.link.fetch-blocks': '获取全部区块',
  'app.welcome.link.block-list': '基于 block 开发，快速构建标准页面',
  'app.documentation': '文档',
  ...pages,
  ...globalHeader,
  ...menu,
  ...settingDrawer,
  ...settings,
  ...pwa,
  ...component,
  ...custom,
  'pages.form.required': '此项为必填项',
  'pages.form.placeholder': '请输入',
  'pages.form.select.placeholder': '请选择',
  'pages.appConfig.base.websiteName': '网站名称',
  'pages.appConfig.base.websiteDescription': '网站描述',
  'pages.appConfig.base.websiteLogo': '网站LOGO',
  'pages.appConfig.base.websiteRecordNumber': '备案编号',
  'pages.appConfig.base.websiteCopyRight': '版权所有',
};
