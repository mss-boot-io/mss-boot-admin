import { queryKeys } from '@mss-admin-core/shared/query/client';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { sessionAPI } from './api';
import type { OnlineSessionListParams } from './contract';
import { onlineSessionRefetchInterval } from './polling';

export function useOnlineSessionPage(params: OnlineSessionListParams) {
  return useQuery({
    queryKey: queryKeys.onlineSessionList(params),
    queryFn: () => sessionAPI.loadPage(params),
    placeholderData: keepPreviousData,
    staleTime: 10_000,
    refetchInterval: (query) => onlineSessionRefetchInterval(query.state.error),
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
  });
}

export function useOnlineSession(id?: string) {
  return useQuery({
    queryKey: queryKeys.onlineSession(id ?? ''),
    queryFn: () => sessionAPI.loadOne(id as string),
    enabled: Boolean(id),
    staleTime: 5_000,
  });
}
