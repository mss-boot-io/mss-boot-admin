import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { operationsAPI } from './operations-api';
import type { KeyListParams, OpsListParams, SalesOrderParams, UserListParams } from './contract';

export const operationsQueryKeys = {
  all: ['litellmops'] as const,
  gatewayHealth: () => ['litellmops', 'gateway', 'health'] as const,
  gatewayModels: () => ['litellmops', 'gateway', 'models'] as const,
  users: (params: UserListParams) => ['litellmops', 'managed-users', params] as const,
  user: (id: string) => ['litellmops', 'managed-user', id] as const,
  recharges: (id: string) => ['litellmops', 'recharges', id] as const,
  keys: (params: KeyListParams) => ['litellmops', 'managed-keys', params] as const,
  products: (params: OpsListParams) => ['litellmops', 'sales-products', params] as const,
  orders: (params: SalesOrderParams) => ['litellmops', 'sales-orders', params] as const,
  order: (id: string) => ['litellmops', 'sales-order', id] as const,
};

export function useGatewayHealth() {
  return useQuery({
    queryKey: operationsQueryKeys.gatewayHealth(),
    queryFn: operationsAPI.gateway.health,
    refetchInterval: 30_000,
  });
}

export function useGatewayModels() {
  return useQuery({
    queryKey: operationsQueryKeys.gatewayModels(),
    queryFn: operationsAPI.gateway.models,
    staleTime: 15_000,
  });
}

export function useManagedUsers(params: UserListParams) {
  return useQuery({
    queryKey: operationsQueryKeys.users(params),
    queryFn: () => operationsAPI.users.list(params),
    placeholderData: keepPreviousData,
  });
}

export function useManagedUser(id?: string) {
  return useQuery({
    queryKey: operationsQueryKeys.user(id ?? ''),
    queryFn: () => operationsAPI.users.detail(id as string),
    enabled: Boolean(id),
  });
}

export function useUserRecharges(id?: string) {
  return useQuery({
    queryKey: operationsQueryKeys.recharges(id ?? ''),
    queryFn: () => operationsAPI.users.recharges(id as string),
    enabled: Boolean(id),
  });
}

export function useManagedKeys(params: KeyListParams) {
  return useQuery({
    queryKey: operationsQueryKeys.keys(params),
    queryFn: () => operationsAPI.keys.list(params),
    placeholderData: keepPreviousData,
  });
}

export function useSalesProducts(params: OpsListParams) {
  return useQuery({
    queryKey: operationsQueryKeys.products(params),
    queryFn: () => operationsAPI.sales.products(params),
    placeholderData: keepPreviousData,
  });
}

export function useSalesOrders(params: SalesOrderParams) {
  return useQuery({
    queryKey: operationsQueryKeys.orders(params),
    queryFn: () => operationsAPI.sales.orders(params),
    placeholderData: keepPreviousData,
    refetchInterval: (query) => {
      const active = query.state.data?.items.some((order) =>
        ['executing', 'applied_unverified'].includes(order.status),
      );
      return active ? 5_000 : false;
    },
  });
}

export function useSalesOrder(id?: string) {
  return useQuery({
    queryKey: operationsQueryKeys.order(id ?? ''),
    queryFn: () => operationsAPI.sales.order(id as string),
    enabled: Boolean(id),
  });
}
