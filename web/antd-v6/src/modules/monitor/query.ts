import { queryKeys } from '@mss-admin-core/shared/query/client';
import { useQuery } from '@tanstack/react-query';
import { monitorAPI } from './api';
import { monitorRefetchInterval } from './polling';

export function useMonitorSnapshot() {
  return useQuery({
    queryKey: queryKeys.monitor,
    queryFn: () => monitorAPI.loadSnapshot(),
    retry: false,
    staleTime: 0,
    refetchInterval: (query) =>
      monitorRefetchInterval(query.state.data, query.state.error, query.state.fetchFailureCount),
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
  });
}
