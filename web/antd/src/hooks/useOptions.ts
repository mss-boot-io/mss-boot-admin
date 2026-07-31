import { useState, useEffect, useCallback } from 'react';
import { getOptions } from '@/services/admin/option';
import { ProSchemaValueEnumObj } from '@ant-design/pro-components';
import { toOptionItems } from '@/util/toOptions';

/**
 * Option query specification
 */
export interface OptionQuery {
  category: string;
  name: string;
}

/**
 * Individual option result data
 */
export interface OptionResult {
  option: API.Option | undefined;
  valueEnum: ProSchemaValueEnumObj;
}

/**
 * useOptions Hook Result
 */
export interface UseOptionsResult {
  options: Record<string, OptionResult>;
  loading: boolean;
  refresh: () => void;
}

const createKey = (query: OptionQuery): string => `${query.category}:${query.name}`;

/**
 * useOptions Hook - Batch fetches multiple options in parallel
 *
 * @param queries - Array of OptionQuery objects specifying category and name
 * @returns UseOptionsResult with options map, loading state, and refresh function
 *
 * @example
 * // Fetch multiple options at once
 * const queries = [
 *   { category: 'user', name: 'status' },
 *   { category: 'system', name: 'type' },
 * ];
 * const { options, loading } = useOptions(queries);
 *
 * // Access individual option by key
 * const statusEnum = options['user:status']?.valueEnum;
 */
export function useOptions(queries: OptionQuery[]): UseOptionsResult {
  const [options, setOptions] = useState<Record<string, OptionResult>>({});
  const [loading, setLoading] = useState(false);

  const fetchOptions = useCallback(async () => {
    if (!queries.length) return;

    setLoading(true);
    try {
      const results = await Promise.all(
        queries.map(async (query) => {
          const res = await getOptions({
            category: query.category,
            name: query.name,
          } as any);
          const option = res.data?.[0];
          const valueEnum = option?.items ? toOptionItems(option.items) : {};
          return {
            key: createKey(query),
            result: {
              option,
              valueEnum,
            },
          };
        }),
      );

      const optionsMap: Record<string, OptionResult> = {};
      results.forEach(({ key, result }) => {
        optionsMap[key] = result;
      });

      setOptions(optionsMap);
    } finally {
      setLoading(false);
    }
  }, [queries]);

  useEffect(() => {
    fetchOptions();
  }, [fetchOptions]);

  return {
    options,
    loading,
    refresh: fetchOptions,
  };
}

export default useOptions;
