import { ConfigProvider } from 'antd';
import type { ReactNode } from 'react';
import { useEffect, useSyncExternalStore } from 'react';
import { createThemeConfig } from '../design-system/theme';
import { getThemeRuntimeSnapshot, subscribeThemeRuntime } from './runtime';

export function ThemeRuntimeProvider({ children }: { children: ReactNode }) {
  const runtime = useSyncExternalStore(
    subscribeThemeRuntime,
    getThemeRuntimeSnapshot,
    getThemeRuntimeSnapshot,
  );

  useEffect(() => {
    const root = document.documentElement;
    const previousFilter = document.body.style.filter;
    root.dataset.mssTheme = runtime.resolved.settings.navTheme;
    root.style.colorScheme = runtime.resolved.settings.navTheme === 'realDark' ? 'dark' : 'light';
    document.body.style.filter = runtime.resolved.settings.colorWeak ? 'invert(80%)' : '';
    return () => {
      document.body.style.filter = previousFilter;
    };
  }, [runtime.resolved.settings.colorWeak, runtime.resolved.settings.navTheme]);

  return (
    <ConfigProvider theme={createThemeConfig(runtime.resolved.settings)}>{children}</ConfigProvider>
  );
}
