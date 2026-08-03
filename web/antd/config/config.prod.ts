// https://umijs.org/config/
import { defineConfig } from '@umijs/max';

export default defineConfig({
  define: {
    API_URL: 'https://admin-api.mss-boot-io.top',
  },
  theme: {
    // 如果不想要 configProvide 动态设置主题需要把这个设置为 default
    // 只有设置为 variable， 才能使用 configProvide 动态设置主色调
    'root-entry-name': 'default',
  },
});
