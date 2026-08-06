import { Grid } from 'antd';
import { useEffect, useState } from 'react';

const getMediaMatch = (query: string): boolean | undefined => {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return undefined;
  }

  return window.matchMedia(query).matches;
};

const useMediaQuery = (query: string): boolean | undefined => {
  const [matches, setMatches] = useState<boolean | undefined>(() => getMediaMatch(query));

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return undefined;
    }

    const mediaQuery = window.matchMedia(query);
    const updateMatches = () => setMatches(mediaQuery.matches);
    updateMatches();

    if (typeof mediaQuery.addEventListener === 'function') {
      mediaQuery.addEventListener('change', updateMatches);
      return () => mediaQuery.removeEventListener('change', updateMatches);
    }

    mediaQuery.addListener(updateMatches);
    return () => mediaQuery.removeListener(updateMatches);
  }, [query]);

  return matches;
};

export interface ResponsiveInfo {
  screens: Partial<Record<string, boolean>>;
  isMobile: boolean;
  isTablet: boolean;
  isDesktop: boolean;
}

export const useResponsive = (): ResponsiveInfo => {
  const screens = Grid.useBreakpoint();
  const mobileMatch = useMediaQuery('(max-width: 767px)');
  const tabletMatch = useMediaQuery('(min-width: 768px) and (max-width: 991px)');
  const isMobile = mobileMatch ?? screens.md === false;
  const isTablet = tabletMatch ?? (screens.md === true && screens.lg === false);

  return {
    screens,
    isMobile,
    isTablet,
    isDesktop: !isMobile && !isTablet,
  };
};

export default useResponsive;
