import { Grid } from 'antd';

export interface ResponsiveInfo {
  screens: Partial<Record<string, boolean>>;
  isMobile: boolean;
  isTablet: boolean;
  isDesktop: boolean;
}

export const useResponsive = (): ResponsiveInfo => {
  const screens = Grid.useBreakpoint();

  return {
    screens,
    isMobile: !screens.md,
    isTablet: !!screens.md && !screens.lg,
    isDesktop: !!screens.lg,
  };
};

export default useResponsive;
