import { renderHook } from '@testing-library/react';
import { useResponsive } from '../useResponsive';

describe('useResponsive', () => {
  it('should return responsive breakpoints', () => {
    const { result } = renderHook(() => useResponsive());

    expect(result.current).toHaveProperty('screens');
    expect(result.current).toHaveProperty('isMobile');
    expect(result.current).toHaveProperty('isTablet');
    expect(result.current).toHaveProperty('isDesktop');
  });

  it('should have screens object with breakpoint flags', () => {
    const { result } = renderHook(() => useResponsive());

    expect(result.current.screens).toBeDefined();
    expect(typeof result.current.screens).toBe('object');
  });

  it('should return boolean for isMobile', () => {
    const { result } = renderHook(() => useResponsive());

    expect(typeof result.current.isMobile).toBe('boolean');
  });

  it('should return boolean for isTablet', () => {
    const { result } = renderHook(() => useResponsive());

    expect(typeof result.current.isTablet).toBe('boolean');
  });

  it('should return boolean for isDesktop', () => {
    const { result } = renderHook(() => useResponsive());

    expect(typeof result.current.isDesktop).toBe('boolean');
  });

  it('uses the current media query result on the first render', () => {
    const originalMatchMedia = window.matchMedia;
    const addEventListener = jest.fn();
    const removeEventListener = jest.fn();

    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: jest.fn((query: string) =>
        ({
          matches: query.includes('max-width: 767px'),
          media: query,
          addEventListener,
          removeEventListener,
          addListener: jest.fn(),
          removeListener: jest.fn(),
          dispatchEvent: jest.fn(),
        }) as unknown as MediaQueryList,
      ),
    });

    const { result, unmount } = renderHook(() => useResponsive());

    expect(result.current.isMobile).toBe(true);
    expect(result.current.isDesktop).toBe(false);

    unmount();
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: originalMatchMedia,
    });
  });
});
