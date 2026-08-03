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
});
