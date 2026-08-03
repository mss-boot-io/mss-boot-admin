import { defineConfig } from 'dumi';

export default defineConfig({
  themeConfig: {
    name: 'mss-boot',
    rtl: true,
    logo: 'favicon.ico',
    footer: `Open-source MIT Licensed | Copyright © 2024-present
    <br />
    Powered by <a target="_blank" href="https://github.com/mss-boot-io">mss-boot-io</av>`,
    socialLinks: {
      github: 'https://github.com/mss-boot-io/mss-boot',
    },
  },
  sitemap: { hostname: 'https://docs.mss-boot-io.top' },
});
