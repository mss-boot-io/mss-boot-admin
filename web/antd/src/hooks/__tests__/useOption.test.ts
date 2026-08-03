import { renderHook, waitFor } from '@testing-library/react';
import { useOption } from '../useOption';
import * as optionService from '@/services/admin/option';

jest.mock('@/services/admin/option');

describe('useOption', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('should return empty valueEnum when no option found', async () => {
    (optionService.getOptions as jest.Mock).mockResolvedValue({
      data: [],
    });

    const { result } = renderHook(() => useOption('system', 'nonexistent'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.option).toBeUndefined();
    expect(result.current.valueEnum).toEqual({});
  });

  it('should not fetch when category or name is empty', () => {
    const { result } = renderHook(() => useOption('', 'status'));

    expect(optionService.getOptions).not.toHaveBeenCalled();
    expect(result.current.loading).toBe(false);
    expect(result.current.valueEnum).toEqual({});
  });
});
