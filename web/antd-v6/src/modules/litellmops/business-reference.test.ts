import { describe, expect, it } from 'vitest';
import {
  availableManagementActions,
  availableOrderActions,
  canReconcileRecharge,
  createBusinessReference,
  extractOneTimeKey,
  isManagementPendingResponse,
  managementMutationOutcome,
  normalizeOptionalSKU,
  rechargeBudgetMicroRange,
} from './business-reference';
import type {
  ManagementCommand,
  RechargeRecord,
  SalesOrder,
  SalesOrderStatus,
  SalesPaymentStatus,
} from './contract';

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

  it('normalizes an omitted optional SKU without throwing', () => {
    expect(normalizeOptionalSKU()).toBe('');
    expect(normalizeOptionalSKU(' sku-1 ')).toBe('sku-1');
  });

  it('requires durable recovery for uncertain management and recharge writes', () => {
    const command = {
      id: 'command-1',
      created_at: '2026-09-11T00:00:00Z',
      status: 'result_unverified',
      requires_manual_review: true,
      last_observed_at: '2026-09-11T00:01:00Z',
    } as ManagementCommand;
    expect(isManagementPendingResponse({ code: 'management_reconcile_required', command })).toBe(
      true,
    );
    expect(
      extractOneTimeKey({ code: 'management_reconcile_required', command } as never),
    ).toBeUndefined();
    expect(managementMutationOutcome({ code: 'management_reconcile_required', command })).toEqual({
      status: 'pending',
      command,
    });
    expect(managementMutationOutcome({ raw_key: 'sk-once' })).toEqual({
      status: 'completed',
      rawKey: 'sk-once',
    });
    expect(availableManagementActions(command, Date.parse('2026-09-11T00:03:00Z'))).toEqual([
      'reconcile',
      'resolve_applied',
      'resolve_not_applied',
    ]);
    expect(
      availableManagementActions(
        { ...command, status: 'executing' },
        Date.parse('2026-09-11T00:03:00Z'),
      ),
    ).toEqual(['reconcile', 'resolve_applied', 'resolve_not_applied']);
    expect(
      availableManagementActions(
        { ...command, status: 'completed' },
        Date.parse('2026-09-11T00:03:00Z'),
      ),
    ).toEqual([]);
    expect(availableManagementActions({ ...command, last_observed_at: null })).toEqual([
      'reconcile',
    ]);
    expect(availableManagementActions(command, Date.parse('2026-09-11T00:01:30Z'))).toEqual([
      'reconcile',
    ]);

    const recharge = {
      status: 'applied_unverified',
      before_budget: 0,
      after_budget: 0,
      before_budget_usd_micro: null,
      target_after_usd_micro: null,
    } as RechargeRecord;
    expect(canReconcileRecharge(recharge)).toBe(true);
    expect(canReconcileRecharge({ ...recharge, status: 'approved' })).toBe(true);
    expect(canReconcileRecharge({ ...recharge, status: 'completed' })).toBe(false);
    expect(rechargeBudgetMicroRange(recharge)).toBeUndefined();
    expect(
      rechargeBudgetMicroRange({
        ...recharge,
        before_budget_usd_micro: 1_000_000,
        target_after_usd_micro: 6_000_000,
      }),
    ).toEqual([1_000_000, 6_000_000]);
  });

  it('offers only state-machine actions and never blindly retries an uncertain write', () => {
    expect(availableOrderActions(order('received'))).toEqual(['verify', 'refund-review']);
    expect(availableOrderActions(order('verified_paid'))).toEqual(['match', 'refund-review']);
    expect(availableOrderActions(order('mapped'))).toEqual(['approve', 'refund-review']);
    expect(availableOrderActions(order('approved'))).toEqual(['execute', 'refund-review']);
    expect(availableOrderActions(order('executing'))).toEqual(['reconcile']);
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
