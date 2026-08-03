import { renderHook, waitFor } from '@testing-library/react';
import { useOptions } from '../useOptions';
import * as optionService from '@/services/admin/option';

jest.mock('@/services/admin/option');

describe('useOptions', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('should handle empty queries array', () => {
    const { result } = renderHook(() => useOptions([]));

    expect(result.current.loading).toBe(false);
    expect(result.current.options).toEqual({});
    expect(optionService.getOptions).not.toHaveBeenCalled();
  });

  it('should provide refresh function', async () => {
    const mockOption: API.Option = {
      id: '1',
      category: 'system',
      name: 'status',
      items: [
        { id: '1', key: 'enabled', label: 'Enabled', value: 'enabled', color: 'green', sort: 1 },
      ],
    };

    (optionService.getOptions as jest.Mock).mockResolvedValue({
      data: [mockOption],
    });

    const queries = [{ category: 'system', name: 'status' }];

    const { result } = renderHook(() => useOptions(queries));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(typeof result.current.refresh).toBe('function');
  });
});
