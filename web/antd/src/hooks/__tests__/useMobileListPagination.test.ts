import { act, renderHook, waitFor } from '@testing-library/react';
import {
  MOBILE_LIST_PAGE_SIZE,
  parseMobilePageResponse,
  useMobileListPagination,
} from '../useMobileListPagination';

type Item = { id: string; name: string };

const makeItems = (start: number, count: number): Item[] =>
  Array.from({ length: count }, (_, index) => ({
    id: String(start + index),
    name: `Item ${start + index}`,
  }));

describe('useMobileListPagination', () => {
  it('normalizes paginated and direct-array responses', () => {
    expect(
      parseMobilePageResponse<Item>({
        data: [{ id: '1', name: 'Item 1' }],
        total: 2,
        current: 1,
        pageSize: 1,
      }),
    ).toEqual({
      items: [{ id: '1', name: 'Item 1' }],
      total: 2,
      isPaginated: true,
    });

    expect(parseMobilePageResponse<Item>([{ id: '1', name: 'Item 1' }])).toEqual({
      items: [{ id: '1', name: 'Item 1' }],
      isPaginated: false,
    });
  });

  it('loads the next server page only when requested', async () => {
    const firstPage = makeItems(1, MOBILE_LIST_PAGE_SIZE);
    const secondPage = makeItems(MOBILE_LIST_PAGE_SIZE + 1, 6);
    const request = jest
      .fn<Promise<unknown>, [{ current: number; pageSize: number }]>()
      .mockResolvedValueOnce({
        data: firstPage,
        total: 30,
        current: 1,
        pageSize: MOBILE_LIST_PAGE_SIZE,
      })
      .mockResolvedValueOnce({
        data: secondPage,
        total: 30,
        current: 2,
        pageSize: MOBILE_LIST_PAGE_SIZE,
      });

    const { result } = renderHook(() => useMobileListPagination<Item>(request));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(request).toHaveBeenCalledWith({ current: 1, pageSize: MOBILE_LIST_PAGE_SIZE });
    expect(result.current.dataSource).toHaveLength(MOBILE_LIST_PAGE_SIZE);
    expect(result.current.hasMore).toBe(true);

    await act(async () => {
      await result.current.loadMore();
    });

    expect(request).toHaveBeenLastCalledWith({ current: 2, pageSize: MOBILE_LIST_PAGE_SIZE });
    expect(request).toHaveBeenCalledTimes(2);
    expect(result.current.dataSource).toHaveLength(30);
    expect(result.current.hasMore).toBe(false);
  });

  it('caps direct-array responses and paginates them without another request', async () => {
    const request = jest
      .fn<Promise<unknown>, [{ current: number; pageSize: number }]>()
      .mockResolvedValue(makeItems(1, MOBILE_LIST_PAGE_SIZE + 1));

    const { result } = renderHook(() => useMobileListPagination<Item>(request));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.dataSource).toHaveLength(MOBILE_LIST_PAGE_SIZE);
    expect(result.current.hasMore).toBe(true);
    expect(result.current.isClientSideFallback).toBe(true);

    await act(async () => {
      await result.current.loadMore();
    });

    expect(request).toHaveBeenCalledTimes(1);
    expect(result.current.dataSource).toHaveLength(MOBILE_LIST_PAGE_SIZE + 1);
    expect(result.current.hasMore).toBe(false);
  });
});
