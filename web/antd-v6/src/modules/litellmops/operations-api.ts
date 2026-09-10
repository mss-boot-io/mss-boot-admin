import { request } from '@umijs/max';
import type {
  GatewayHealth,
  GatewayModel,
  KeyListParams,
  ManagedKey,
  ManagedKeyInput,
  ManagedKeyPatch,
  ManagedUser,
  ManagedUserDetail,
  ManagedUserInput,
  ManagedUserPatch,
  ManagementAware,
  ManagementCommand,
  ManagementCommandParams,
  ManagementResolveRequest,
  OneTimeKeyResult,
  OpsPage,
  RechargeRecord,
  RechargeRequest,
  SalesOrder,
  SalesOrderInput,
  SalesOrderParams,
  SalesProduct,
  SalesProductInput,
  SalesProductParams,
  SalesProductPatch,
  UserListParams,
} from './contract';

const base = '/litellmops';

function action(path: string, data?: unknown) {
  return request(path, { method: 'POST', data, skipErrorHandler: true });
}

export const operationsAPI = {
  gateway: {
    health: (): Promise<GatewayHealth> =>
      request(`${base}/gateway/health`, { skipErrorHandler: true }),
    models: (): Promise<{ items: GatewayModel[]; total: number }> =>
      request(`${base}/gateway/models`, { skipErrorHandler: true }),
  },
  users: {
    list: (params: UserListParams): Promise<OpsPage<ManagedUser>> =>
      request(`${base}/users`, { params, skipErrorHandler: true }),
    detail: (id: string): Promise<ManagedUserDetail> =>
      request(`${base}/users/${encodeURIComponent(id)}`, { skipErrorHandler: true }),
    create: (data: ManagedUserInput): Promise<ManagementAware<{ user: unknown }>> =>
      request(`${base}/users`, { method: 'POST', data, skipErrorHandler: true }),
    invite: (data: ManagedUserInput): Promise<ManagementAware<{ user: unknown }>> =>
      action(`${base}/users/invite`, data),
    update: (id: string, data: ManagedUserPatch): Promise<ManagementAware<{ user: unknown }>> =>
      request(`${base}/users/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        data,
        skipErrorHandler: true,
      }),
    remove: (id: string): Promise<ManagementAware<void>> =>
      request(`${base}/users/${encodeURIComponent(id)}`, {
        method: 'DELETE',
        skipErrorHandler: true,
      }),
    block: (id: string): Promise<ManagementAware<{ user: unknown }>> =>
      action(`${base}/users/${encodeURIComponent(id)}/block`),
    unblock: (id: string): Promise<ManagementAware<{ user: unknown }>> =>
      action(`${base}/users/${encodeURIComponent(id)}/unblock`),
    recharge: (id: string, data: RechargeRequest): Promise<RechargeRecord> =>
      action(`${base}/users/${encodeURIComponent(id)}/recharge`, data),
    recharges: (id: string): Promise<{ items: RechargeRecord[] }> =>
      request(`${base}/users/${encodeURIComponent(id)}/recharges`, { skipErrorHandler: true }),
    sync: (): Promise<{
      users: number;
      keys: number;
      users_retired: number;
      keys_retired: number;
    }> => action(`${base}/sync`),
  },
  keys: {
    list: (params: KeyListParams): Promise<OpsPage<ManagedKey>> =>
      request(`${base}/keys`, { params, skipErrorHandler: true }),
    create: (data: ManagedKeyInput): Promise<ManagementAware<OneTimeKeyResult>> =>
      request(`${base}/keys`, { method: 'POST', data, skipErrorHandler: true }),
    update: (id: string, data: ManagedKeyPatch): Promise<ManagementAware<unknown>> =>
      request(`${base}/keys/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        data,
        skipErrorHandler: true,
      }),
    remove: (id: string): Promise<ManagementAware<void>> =>
      request(`${base}/keys/${encodeURIComponent(id)}`, {
        method: 'DELETE',
        skipErrorHandler: true,
      }),
    block: (id: string): Promise<ManagementAware<unknown>> =>
      action(`${base}/keys/${encodeURIComponent(id)}/block`),
    unblock: (id: string): Promise<ManagementAware<unknown>> =>
      action(`${base}/keys/${encodeURIComponent(id)}/unblock`),
    rotate: (id: string): Promise<ManagementAware<OneTimeKeyResult>> =>
      action(`${base}/keys/${encodeURIComponent(id)}/rotate`),
    resetSpend: (id: string): Promise<ManagementAware<unknown>> =>
      action(`${base}/keys/${encodeURIComponent(id)}/reset-spend`, { reset_to_usd_micro: 0 }),
  },
  recharges: {
    reconcile: (id: string): Promise<RechargeRecord> =>
      action(`${base}/recharges/${encodeURIComponent(id)}/reconcile`),
  },
  management: {
    commands: (params: ManagementCommandParams): Promise<OpsPage<ManagementCommand>> =>
      request(`${base}/management/commands`, { params, skipErrorHandler: true }),
    reconcile: (id: string): Promise<ManagementCommand> =>
      action(`${base}/management/commands/${encodeURIComponent(id)}/reconcile`),
    resolve: (id: string, data: ManagementResolveRequest): Promise<ManagementCommand> =>
      action(`${base}/management/commands/${encodeURIComponent(id)}/resolve`, data),
  },
  sales: {
    products: (params: SalesProductParams): Promise<OpsPage<SalesProduct>> =>
      request(`${base}/sales/products`, { params, skipErrorHandler: true }),
    createProduct: (data: SalesProductInput): Promise<SalesProduct> =>
      request(`${base}/sales/products`, { method: 'POST', data, skipErrorHandler: true }),
    updateProduct: (id: string, data: SalesProductPatch): Promise<SalesProduct> =>
      request(`${base}/sales/products/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        data,
        skipErrorHandler: true,
      }),
    orders: (params: SalesOrderParams): Promise<OpsPage<SalesOrder>> =>
      request(`${base}/sales/orders`, { params, skipErrorHandler: true }),
    order: (id: string): Promise<SalesOrder> =>
      request(`${base}/sales/orders/${encodeURIComponent(id)}`, { skipErrorHandler: true }),
    createOrder: (data: SalesOrderInput): Promise<SalesOrder> =>
      request(`${base}/sales/orders`, { method: 'POST', data, skipErrorHandler: true }),
    verify: (id: string, note?: string): Promise<SalesOrder> =>
      action(`${base}/sales/orders/${encodeURIComponent(id)}/verify`, { note }),
    match: (id: string, data: { product_id: string; user_email: string }): Promise<SalesOrder> =>
      action(`${base}/sales/orders/${encodeURIComponent(id)}/match`, data),
    approve: (id: string, note?: string): Promise<SalesOrder> =>
      action(`${base}/sales/orders/${encodeURIComponent(id)}/approve`, { note }),
    execute: (id: string): Promise<SalesOrder> =>
      action(`${base}/sales/orders/${encodeURIComponent(id)}/execute`),
    reconcile: (id: string): Promise<SalesOrder> =>
      action(`${base}/sales/orders/${encodeURIComponent(id)}/reconcile`),
    reviewRefund: (id: string, data: { reason: string }): Promise<SalesOrder> =>
      action(`${base}/sales/orders/${encodeURIComponent(id)}/refund-review`, data),
  },
};
