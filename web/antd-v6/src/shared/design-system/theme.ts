import type { ThemeConfig } from 'antd';
import { theme } from 'antd';
import type { ThemeSettings } from '../theme/contract';

export function createThemeConfig(settings: ThemeSettings): ThemeConfig {
  return {
    algorithm: settings.navTheme === 'realDark' ? theme.darkAlgorithm : theme.defaultAlgorithm,
    cssVar: { prefix: 'mss' },
    token: {
      colorPrimary: settings.colorPrimary,
      borderRadius: 8,
      fontFamily:
        "Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    },
    components: {
      Button: { controlHeight: 36 },
      Card: { headerFontSize: 16 },
      Table: { headerBg: 'var(--mss-color-fill-quaternary)' },
    },
  };
}
