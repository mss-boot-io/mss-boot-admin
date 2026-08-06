import { useCallback, useEffect, useRef, useState } from 'react';

export const MOBILE_LIST_PAGE_SIZE = 24;

type PageResponse<T> = {
  data?: T[];
  total?: number;
  current?: number;
  pageSize?: number;
};

type RecordLike = Record<string, unknown>;

export interface MobilePage<T> {
  items: T[];
  total?: number;
  isPaginated: boolean;
}

export interface UseMobileListPaginationResult<T> {
  dataSource: T[];
  error?: Error;
  hasMore: boolean;
  isClientSideFallback: boolean;
  loading: boolean;
  loadingMore: boolean;
  loadMore: () => Promise<void>;
  reload: () => Promise<void>;
}

const isRecord = (value: unknown): value is RecordLike =>
  typeof value === 'object' && value !== null;

const toFiniteNumber = (value: unknown): number | undefined => {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }

  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : undefined;
  }

  return undefined;
};

const getPageMetadata = (value: RecordLike) => {
  const total = toFiniteNumber(value.total);
  const current = toFiniteNumber(value.current);
  const pageSize = toFiniteNumber(value.pageSize);

  return {
    total,
    isPaginated: total !== undefined || current !== undefined || pageSize !== undefined,
  };
};

/**
 * Normalizes the two response forms used by the management APIs:
 * paginated { data, total, current, pageSize } responses and direct arrays.
 */
export const parseMobilePageResponse = <T,>(response: unknown): MobilePage<T> => {
  if (Array.isArray(response)) {
    return { items: response as T[], isPaginated: false };
  }

  if (!isRecord(response)) {
    return { items: [], isPaginated: false };
  }

  if (Array.isArray(response.data)) {
    const metadata = getPageMetadata(response);
    return {
      items: response.data as T[],
      total: metadata.total,
      isPaginated: metadata.isPaginated,
    };
  }

  if (isRecord(response.data) && Array.isArray(response.data.data)) {
    const nested = response.data as PageResponse<T>;
    const metadata = getPageMetadata(nested as RecordLike);
    return {
      items: nested.data || [],
      total: metadata.total,
      isPaginated: metadata.isPaginated,
    };
  }

  return { items: [], isPaginated: false };
};

const mergePageItems = <T extends { id?: unknown }>(current: T[], next: T[]): T[] => {
  const ids = new Set(
    current
      .map((item) => item.id)
      .filter((id): id is string | number => typeof id === 'string' || typeof id === 'number'),
  );

  return current.concat(
    next.filter((item) => {
      if (typeof item.id !== 'string' && typeof item.id !== 'number') {
        return true;
      }

      if (ids.has(item.id)) {
        return false;
      }

      ids.add(item.id);
      return true;
    }),
  );
};

/**
 * Incrementally loads mobile list data. APIs that still return a direct array
 * are displayed in client-side pages so they do not mount every card at once.
 */
export const useMobileListPagination = <T extends { id?: unknown }>(
  request: (params: { current: number; pageSize: number }) => Promise<unknown>,
  pageSize = MOBILE_LIST_PAGE_SIZE,
): UseMobileListPaginationResult<T> => {
  const [dataSource, setDataSource] = useState<T[]>([]);
  const [error, setError] = useState<Error>();
  const [hasMore, setHasMore] = useState(false);
  const [isClientSideFallback, setIsClientSideFallback] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const dataSourceRef = useRef<T[]>([]);
  const currentPageRef = useRef(0);
  const requestVersionRef = useRef(0);
  const clientSideItemsRef = useRef<T[]>();
  const loadingMoreRef = useRef(false);
  const mountedRef = useRef(true);

  const loadPage = useCallback(
    async (page: number, replace: boolean) => {
      const cachedItems = clientSideItemsRef.current;
      if (!replace && cachedItems) {
        const items = cachedItems.slice(0, page * pageSize);
        dataSourceRef.current = items;
        setDataSource(items);
        setError(undefined);
        setHasMore(items.length < cachedItems.length);
        setIsClientSideFallback(true);
        currentPageRef.current = page;
        return;
      }

      if (!replace && loadingMoreRef.current) {
        return;
      }

      const requestVersion = ++requestVersionRef.current;
      if (replace) {
        setLoading(true);
      } else {
        loadingMoreRef.current = true;
        setLoadingMore(true);
      }
      setError(undefined);

      try {
        const response = await request({ current: page, pageSize });
        if (!mountedRef.current || requestVersion !== requestVersionRef.current) {
          return;
        }

        const result = parseMobilePageResponse<T>(response);
        if (!result.isPaginated) {
          clientSideItemsRef.current = result.items;
          const items = result.items.slice(0, page * pageSize);
          dataSourceRef.current = items;
          setDataSource(items);
          setHasMore(items.length < result.items.length);
          setIsClientSideFallback(true);
        } else {
          clientSideItemsRef.current = undefined;
          const items = replace
            ? result.items
            : mergePageItems(dataSourceRef.current, result.items);
          dataSourceRef.current = items;
          setDataSource(items);
          setHasMore(
            result.total !== undefined
              ? items.length < result.total
              : result.items.length === pageSize,
          );
          setIsClientSideFallback(false);
        }
        currentPageRef.current = page;
      } catch (requestError) {
        if (!mountedRef.current || requestVersion !== requestVersionRef.current) {
          return;
        }

        setError(
          requestError instanceof Error
            ? requestError
            : new Error('Unable to load the list. Please retry.'),
        );
      } finally {
        if (!replace) {
          loadingMoreRef.current = false;
        }
        if (mountedRef.current && requestVersion === requestVersionRef.current) {
          setLoading(false);
          setLoadingMore(false);
        }
      }
    },
    [pageSize, request],
  );

  const reload = useCallback(async () => {
    clientSideItemsRef.current = undefined;
    currentPageRef.current = 0;
    await loadPage(1, true);
  }, [loadPage]);

  const loadMore = useCallback(async () => {
    if (loading || loadingMore || loadingMoreRef.current || !hasMore) {
      return;
    }

    await loadPage(currentPageRef.current + 1, false);
  }, [hasMore, loadPage, loading, loadingMore]);

  useEffect(() => {
    mountedRef.current = true;

    return () => {
      mountedRef.current = false;
      requestVersionRef.current += 1;
    };
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  return {
    dataSource,
    error,
    hasMore,
    isClientSideFallback,
    loading,
    loadingMore,
    loadMore,
    reload,
  };
};

export default useMobileListPagination;
