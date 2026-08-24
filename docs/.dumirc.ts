import { defineConfig } from 'dumi';

export default defineConfig({
  themeConfig: {
    name: 'mss-boot Admin',
    rtl: false,
    logo: '/favicon.ico',
    nav: [
      {
        title: '快速开始',
        link: '/getting-started',
        activePath: '/getting-started',
      },
      { title: '使用指南', link: '/guide', activePath: '/guide' },
      { title: 'Admin', link: '/admin', activePath: '/admin' },
      { title: 'Agent 开发', link: '/agent', activePath: '/agent' },
      {
        title: '架构',
        link: '/architecture/agent-native-foundation',
        activePath: '/architecture',
      },
      { title: '发布', link: '/releases', activePath: '/releases' },
      {
        title: '更多',
        children: [
          { title: '包与导入', link: '/getting-started/packages' },
          { title: 'mss-shop 范本', link: '/getting-started/mss-shop' },
          { title: 'Supplier 模块', link: '/modules/supplier' },
          { title: '参与贡献', link: '/coding/first-contribution' },
          { title: '安全策略 FAQ', link: '/devops/security-policy-faq' },
        ],
      },
    ],
    footer: `Open-source MIT Licensed | Copyright © 2024-present
    <br />
    Powered by <a target="_blank" href="https://github.com/mss-boot-io">mss-boot-io</a>`,
    socialLinks: {
      github: 'https://github.com/mss-boot-io/mss-boot-admin',
    },
  },
  sitemap: { hostname: 'https://docs.mss-boot-io.top' },
});
