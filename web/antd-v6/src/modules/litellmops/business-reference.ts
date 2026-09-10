import type { OneTimeKeyResult, SalesOrder, SalesOrderStatus } from './contract';

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
