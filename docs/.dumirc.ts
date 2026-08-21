import { defineConfig } from 'dumi';

export default defineConfig({
  themeConfig: {
    name: 'mss-boot Admin',
    rtl: false,
    logo: '/favicon.ico',
    // Keep only stable, high-frequency journeys at the top level. New sections
    // belong under “更多” by default so the header stays bounded as docs grow.
    nav: [
      { title: 'Admin', link: '/admin', activePath: '/admin' },
      { title: '指南', link: '/guide', activePath: '/guide' },
      { title: 'Agent 开发', link: '/agent', activePath: '/agent' },
      {
        title: '架构',
        link: '/architecture/agent-native-foundation',
        activePath: '/architecture',
      },
      {
        title: '发布',
        link: '/releases',
        activePath: '/releases',
      },
      {
        title: '更多',
        children: [
          { title: 'coding', link: '/coding', activePath: '/coding' },
          { title: 'devops', link: '/devops', activePath: '/devops' },
          {
            title: 'Business modules',
            link: '/modules/supplier',
            activePath: '/modules',
          },
          { title: 'aigc', link: '/aigc', activePath: '/aigc' },
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
