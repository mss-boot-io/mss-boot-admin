import type {
  ManagementCommand,
  ManagementPendingResponse,
  OneTimeKeyResult,
  RechargeRecord,
  SalesOrder,
  SalesOrderStatus,
} from './contract';

export function createBusinessReference(
  prefix: string,
  now = new Date(),
  unique = globalThis.crypto.randomUUID(),
): string {
  const timestamp = now
    .toISOString()
    .replace(/[-:.TZ]/g, '')
    .slice(0, 14);
  const safePrefix =
    prefix
      .toLowerCase()
      .replace(/[^a-z0-9_-]+/g, '-')
      .replace(/^-|-$/g, '') || 'manual';
  return `${safePrefix}-${timestamp}-${unique.slice(0, 12)}`;
}

export function extractOneTimeKey(result: OneTimeKeyResult): string | undefined {
  if (typeof result.raw_key === 'string' && result.raw_key) return result.raw_key;
  if (typeof result.token === 'string' && result.token) return result.token;
  return typeof result.key === 'string' && result.key ? result.key : undefined;
}

export function normalizeOptionalSKU(value?: string): string {
  return (value ?? '').trim();
}

export function isManagementPendingResponse(value: unknown): value is ManagementPendingResponse {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Partial<ManagementPendingResponse>;
  return candidate.code === 'management_reconcile_required' && Boolean(candidate.command?.id);
}

export function managementMutationOutcome(
  value: unknown,
): { status: 'pending'; command: ManagementCommand } | { status: 'completed'; rawKey?: string } {
  if (isManagementPendingResponse(value)) return { status: 'pending', command: value.command };
  const rawKey =
    value && typeof value === 'object' ? extractOneTimeKey(value as OneTimeKeyResult) : undefined;
  return { status: 'completed', rawKey };
}

export function isPendingManagementCommand(command: ManagementCommand): boolean {
  return command.status === 'executing' || command.status === 'result_unverified';
}

export type ManagementCommandAction = 'reconcile' | 'resolve_applied' | 'resolve_not_applied';

export function availableManagementActions(
  command: ManagementCommand,
  now = Date.now(),
): ManagementCommandAction[] {
  if (!isPendingManagementCommand(command)) return [];
  const createdAt = Date.parse(command.created_at);
  const observedAt = Date.parse(command.last_observed_at ?? '');
  const manualResolutionReady =
    Number.isFinite(createdAt) &&
    Number.isFinite(observedAt) &&
    now - createdAt >= 120_000 &&
    now - observedAt >= 5_000;
  return command.requires_manual_review && manualResolutionReady
    ? ['reconcile', 'resolve_applied', 'resolve_not_applied']
    : ['reconcile'];
}

export function canReconcileRecharge(record: RechargeRecord): boolean {
  if (record.source === 'sales_order') return false;
  return [
    'approved',
    'executing',
    'applied_unverified',
    'retryable_failed',
    'reconcile_required',
  ].includes(record.status);
}

export function rechargeBudgetMicroRange(record: RechargeRecord): [number, number] | undefined {
  if (record.before_budget_usd_micro == null || record.target_after_usd_micro == null)
    return undefined;
  return [record.before_budget_usd_micro, record.target_after_usd_micro];
}

export type SalesOrderAction =
  | 'verify'
  | 'match'
  | 'approve'
  | 'execute'
  | 'reconcile'
  | 'refund-review';

const actionByStatus: Partial<Record<SalesOrderStatus, SalesOrderAction[]>> = {
  received: ['verify'],
  verified_paid: ['match'],
  mapped: ['approve'],
  approved: ['execute'],
  executing: ['reconcile'],
  applied_unverified: ['reconcile'],
  retryable_failed: ['reconcile'],
  reconcile_required: ['reconcile'],
};

const refundReviewStatuses = new Set<SalesOrderStatus>([
  'received',
  'verified_paid',
  'mapped',
  'approved',
  'completed',
  'terminal_failed',
]);

export function availableOrderActions(order: SalesOrder): SalesOrderAction[] {
  const actions = order.payment_status === 'paid' ? [...(actionByStatus[order.status] ?? [])] : [];
  if (refundReviewStatuses.has(order.status)) actions.push('refund-review');
  return actions;
}
