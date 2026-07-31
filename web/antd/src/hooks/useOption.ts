import { useRequest } from '@umijs/max';
import { getOptions } from '@/services/admin/option';
import { ProSchemaValueEnumObj } from '@ant-design/pro-components';
import { toOptionItems } from '@/util/toOptions';

/**
 * useOption Hook Result
 */
export interface UseOptionResult {
  /** The full option object from backend */
  option: API.Option | undefined;
  /** Converted valueEnum for ProComponents */
  valueEnum: ProSchemaValueEnumObj;
  /** Loading state */
  loading: boolean;
  /** Error state */
  error: Error | undefined;
  /** Refresh function to refetch data */
  refresh: () => void;
}

/**
 * useOption Hook - Fetches a single option by category and name
 *
 * @param category - The option category
 * @param name - The option name
 * @returns UseOptionResult with option data, valueEnum, loading, error, and refresh
 *
 * @example
 * // Basic usage
 * const { valueEnum, loading } = useOption('user', 'status');
 *
 * @example
 * // In ProTable columns
 * const { valueEnum } = useOption('system', 'status');
 * const columns = [
 *   {
 *     title: 'Status',
 *     dataIndex: 'status',
 *     valueEnum: valueEnum,
 *   }
 * ];
 */
export function useOption(category: string, name: string): UseOptionResult {
  const {
    data: option,
    loading,
    error,
    refresh,
  } = useRequest(
    async () => {
      const res = await getOptions({
        category,
        name,
      } as any);
      return res.data?.[0];
    },
    {
      cacheKey: `option-${category}:${name}`,
      refreshDeps: [category, name],
      ready: !!category && !!name,
    },
  );

  // Convert OptionItems to ProSchemaValueEnumObj
  const valueEnum: ProSchemaValueEnumObj = option?.items ? toOptionItems(option.items) : {};

  return {
    option,
    valueEnum,
    loading,
    error,
    refresh,
  };
}

export default useOption;
