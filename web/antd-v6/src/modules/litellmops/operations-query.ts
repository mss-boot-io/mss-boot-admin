import { keepPreviousData, useQuery } from '@tanstack/react-query';
import type {
  KeyListParams,
  ManagementCommand,
  ManagementCommandParams,
  ManagementCommandStatus,
  SalesOrderParams,
  SalesProductParams,
  UserListParams,
} from './contract';
import { operationsAPI } from './operations-api';

export const operationsQueryKeys = {
  all: ['litellmops'] as const,
  gatewayHealth: () => ['litellmops', 'gateway', 'health'] as const,
  gatewayModels: () => ['litellmops', 'gateway', 'models'] as const,
  users: (params: UserListParams) => ['litellmops', 'managed-users', params] as const,
  user: (id: string) => ['litellmops', 'managed-user', id] as const,
  recharges: (id: string) => ['litellmops', 'recharges', id] as const,
  keys: (params: KeyListParams) => ['litellmops', 'managed-keys', params] as const,
  products: (params: SalesProductParams) => ['litellmops', 'sales-products', params] as const,
  orders: (params: SalesOrderParams) => ['litellmops', 'sales-orders', params] as const,
  order: (id: string) => ['litellmops', 'sales-order', id] as const,
  managementCommands: (params: ManagementCommandParams) =>
    ['litellmops', 'management-commands', params] as const,
  pendingManagementCommands: () => ['litellmops', 'management-commands', 'pending'] as const,
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
    refetchInterval: (query) => {
      const pending = query.state.data?.items.some((record) =>
        ['executing', 'applied_unverified', 'retryable_failed', 'reconcile_required'].includes(
          record.status,
        ),
      );
      return pending ? 5_000 : false;
    },
  });
}

export function useManagedKeys(params: KeyListParams) {
  return useQuery({
    queryKey: operationsQueryKeys.keys(params),
    queryFn: () => operationsAPI.keys.list(params),
    placeholderData: keepPreviousData,
  });
}

export function useSalesProducts(params: SalesProductParams) {
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

export function useManagementCommands(params: ManagementCommandParams, enabled = true) {
  return useQuery({
    queryKey: operationsQueryKeys.managementCommands(params),
    queryFn: () => operationsAPI.management.commands(params),
    enabled,
    placeholderData: keepPreviousData,
    refetchInterval: (query) => {
      const pending = query.state.data?.items.some(
        (command) => command.status === 'executing' || command.status === 'result_unverified',
      );
      return pending ? 5_000 : false;
    },
  });
}

async function loadManagementStatus(status: ManagementCommandStatus) {
  const items: ManagementCommand[] = [];
  for (let page = 1; ; page += 1) {
    const result = await operationsAPI.management.commands({ page, page_size: 100, status });
    items.push(...result.items);
    if (items.length >= result.total || result.items.length === 0) return items;
  }
}

export function usePendingManagementCommands(enabled = true) {
  return useQuery({
    queryKey: operationsQueryKeys.pendingManagementCommands(),
    queryFn: async () => {
      const groups = await Promise.all([
        loadManagementStatus('executing'),
        loadManagementStatus('result_unverified'),
      ]);
      const unique = new Map(groups.flat().map((command) => [command.id, command]));
      return [...unique.values()].sort(
        (left, right) => Date.parse(right.created_at) - Date.parse(left.created_at),
      );
    },
    enabled,
    refetchInterval: (query) => (query.state.data?.length ? 5_000 : false),
  });
}
