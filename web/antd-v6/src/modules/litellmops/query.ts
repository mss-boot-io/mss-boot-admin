import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { litellmopsAPI } from './api';
import type { BillsParams, KeyPageParams, UserPageParams } from './types';

export const litellmopsQueryKeys = {
  users: (params: UserPageParams) => ['litellmops', 'users', params] as const,
  userDetail: (id: string) => ['litellmops', 'user', id] as const,
  keys: (params: KeyPageParams) => ['litellmops', 'keys', params] as const,
  bills: (params: BillsParams) => ['litellmops', 'bills', params] as const,
};

export function useUserPage(params: UserPageParams) {
  return useQuery({
    queryKey: litellmopsQueryKeys.users(params),
    queryFn: () => litellmopsAPI.listUsers(params),
    placeholderData: keepPreviousData,
    staleTime: 15_000,
  });
}

export function useUserDetail(id?: string) {
  return useQuery({
    queryKey: litellmopsQueryKeys.userDetail(id ?? ''),
    queryFn: () => litellmopsAPI.getUser(id as string),
    enabled: Boolean(id),
  });
}

export function useKeyPage(params: KeyPageParams) {
  return useQuery({
    queryKey: litellmopsQueryKeys.keys(params),
    queryFn: () => litellmopsAPI.listKeys(params),
    placeholderData: keepPreviousData,
    staleTime: 15_000,
  });
}

export function useBillsPage(params: BillsParams) {
  return useQuery({
    queryKey: litellmopsQueryKeys.bills(params),
    queryFn: () => litellmopsAPI.listBills(params),
    placeholderData: keepPreviousData,
    staleTime: 15_000,
  });
}

export function useSyncMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => litellmopsAPI.sync(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['litellmops'] });
    },
  });
}
