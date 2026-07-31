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
  'pages.generator.steps.template.title': '设置模板仓库',
  'pages.generator.steps.template.desc': '这里填入模板仓库链接',
  'pages.generator.steps.branch.title': '选择分支',
  'pages.generator.steps.branch.desc': '这里选择仓库分支',
  'pages.generator.steps.path.title': '选择目录',
  'pages.generator.steps.path.desc': '这里选择模板目录',
  'pages.generator.steps.params.title': '填写参数',
  'pages.generator.steps.params.desc': '这里填写模板参数',
  'pages.generator.steps.params.tooltip': '填写用于模板渲染的变量参数',
  'pages.generator.repo': '目标仓库',
  'pages.generator.repo.tooltip': '生成代码目标仓库地址',
  'pages.generator.service': '服务目录',
  'pages.generator.service.tooltip': '生成代码目标仓库目录',
  'pages.generator.email': '提交邮箱',
  'pages.generator.email.tooltip': '代码仓库提交邮箱',
  'pages.generator.githubAuth': '获取GitHub权限',
  'pages.generator.success': '代码生成成功, 分支为: {branch}',
};
