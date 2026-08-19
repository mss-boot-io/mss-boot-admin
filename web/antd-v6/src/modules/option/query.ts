import { queryKeys } from '@mss-admin-core/shared/query/client';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { optionAPI } from './api';
import type { OptionListParams } from './contract';

export function useOptionPage(params: OptionListParams) {
  return useQuery({
    queryKey: queryKeys.optionList(params),
    queryFn: () => optionAPI.loadPage(params),
    placeholderData: keepPreviousData,
    staleTime: 15_000,
  });
}

export function useOption(id?: string) {
  return useQuery({
    queryKey: queryKeys.option(id ?? ''),
    queryFn: () => optionAPI.loadOne(id as string),
    enabled: Boolean(id),
    staleTime: 30_000,
  });
}
