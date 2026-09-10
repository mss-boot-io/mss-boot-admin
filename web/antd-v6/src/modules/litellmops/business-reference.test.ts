import { describe, expect, it } from 'vitest';
import {
  availableOrderActions,
  createBusinessReference,
  extractOneTimeKey,
} from './business-reference';
import type { SalesOrder, SalesOrderStatus, SalesPaymentStatus } from './contract';

function order(status: SalesOrderStatus, payment_status: SalesPaymentStatus = 'paid'): SalesOrder {
  return {
    id: 'order-1',
    channel: 'xianyu',
    shop: 'shop',
    external_order_id: 'external-1',
    adjustment_type: 'credit',
    user_email: 'buyer@example.com',
    paid_cny_fen: 1_000,
    credit_usd_micro: 5_000_000,
    source_trust: 'manual',
    payment_status,
    status,
    created_at: '2026-09-10T01:02:03Z',
    updated_at: '2026-09-10T01:02:03Z',
  };
}

describe('operations business safety helpers', () => {
  it('creates one deterministic business reference that can be reused for retries', () => {
    const reference = createBusinessReference(
      'Manual Credit',
      new Date('2026-09-10T01:02:03.000Z'),
      '12345678-90ab-cdef-1234-567890abcdef',
    );
    expect(reference).toBe('manual-credit-20260910010203-12345678-90a');
  });

  it('extracts only a one-time raw key response', () => {
    expect(extractOneTimeKey({ raw_key: 'sk-raw', token: 'sk-token' })).toBe('sk-raw');
    expect(extractOneTimeKey({ token: 'sk-token' })).toBe('sk-token');
    expect(extractOneTimeKey({ key: { id: 'record' } as never })).toBeUndefined();
  });

  it('offers only state-machine actions and never blindly retries an uncertain write', () => {
    expect(availableOrderActions(order('received'))).toEqual(['verify', 'refund-review']);
    expect(availableOrderActions(order('verified_paid'))).toEqual(['match', 'refund-review']);
    expect(availableOrderActions(order('mapped'))).toEqual(['approve', 'refund-review']);
    expect(availableOrderActions(order('approved'))).toEqual(['execute', 'refund-review']);
    expect(availableOrderActions(order('executing'))).toEqual([]);
    expect(availableOrderActions(order('applied_unverified'))).toEqual(['reconcile']);
    expect(availableOrderActions(order('retryable_failed'))).toEqual(['reconcile']);
    expect(availableOrderActions(order('reconcile_required'))).toEqual(['reconcile']);
    expect(availableOrderActions(order('completed'))).toEqual(['refund-review']);
    expect(availableOrderActions(order('terminal_failed'))).toEqual(['refund-review']);
    expect(availableOrderActions(order('refund_review', 'refunded'))).toEqual([]);
    expect(availableOrderActions(order('reversed', 'refunded'))).toEqual([]);
    expect(availableOrderActions(order('received', 'unpaid'))).toEqual(['refund-review']);
  });
});
