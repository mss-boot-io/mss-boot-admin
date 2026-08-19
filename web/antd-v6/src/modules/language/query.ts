import { queryKeys } from '@mss-admin-core/shared/query/client';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { languageAPI } from './api';
import type { LanguageListParams } from './contract';

export function useLanguagePage(params: LanguageListParams) {
  return useQuery({
    queryKey: queryKeys.languageList(params),
    queryFn: () => languageAPI.loadPage(params),
    placeholderData: keepPreviousData,
    staleTime: 15_000,
  });
}

export function useLanguage(id?: string) {
  return useQuery({
    queryKey: queryKeys.language(id ?? ''),
    queryFn: () => languageAPI.loadOne(id as string),
    enabled: Boolean(id),
    staleTime: 30_000,
  });
}
