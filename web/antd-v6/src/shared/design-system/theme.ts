import type { ThemeConfig } from 'antd';
import { theme } from 'antd';

export type MSSThemeMode = 'light' | 'dark';

export interface MSSThemeSettings {
  mode: MSSThemeMode;
  colorPrimary: string;
  colorWeak: boolean;
}

export const defaultThemeSettings: MSSThemeSettings = {
  mode: 'light',
  colorPrimary: '#1677ff',
  colorWeak: false,
};

export function createThemeConfig(settings: MSSThemeSettings): ThemeConfig {
  return {
    algorithm: settings.mode === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm,
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
