// The downloadable release is deployed next to the Admin API. Keep requests
// same-origin so the archive works behind any hostname or reverse proxy.
import { defineConfig } from '@umijs/max';

export default defineConfig({
  define: {
    API_URL: '',
  },
});
