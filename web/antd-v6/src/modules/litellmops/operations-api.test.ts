import { request } from '@umijs/max';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { litellmopsAPI } from './api';
import { managementMutationOutcome } from './business-reference';
import { operationsAPI } from './operations-api';

vi.mock('@umijs/max', () => ({ request: vi.fn() }));

const requestMock = vi.mocked(request);

describe('LiteLLM operations API', () => {
  beforeEach(() => {
    requestMock.mockReset();
    requestMock.mockResolvedValue({});
  });

  it('keeps key lifecycle mutations on protected relative endpoints', async () => {
    await operationsAPI.keys.rotate('unsafe/id');
    await operationsAPI.keys.resetSpend('unsafe/id');
    await operationsAPI.keys.remove('unsafe/id');

    expect(requestMock).toHaveBeenNthCalledWith(1, '/litellmops/keys/unsafe%2Fid/rotate', {
      method: 'POST',
      data: undefined,
      skipErrorHandler: true,
    });
    expect(requestMock).toHaveBeenNthCalledWith(2, '/litellmops/keys/unsafe%2Fid/reset-spend', {
      method: 'POST',
      data: { reset_to_usd_micro: 0 },
      skipErrorHandler: true,
    });
    expect(requestMock).toHaveBeenNthCalledWith(3, '/litellmops/keys/unsafe%2Fid', {
      method: 'DELETE',
      skipErrorHandler: true,
    });
  });

  it('quarantines uncertain key issue and rotate results instead of treating them as one-time secrets', async () => {
    const pending = {
      code: 'management_reconcile_required',
      error: 'upstream result unknown',
      command: { id: 'command-1', status: 'result_unverified' },
    };
    requestMock.mockResolvedValue(pending);

    const issued = await operationsAPI.keys.create({
      alias: 'buyer',
      user_email: 'buyer@example.com',
      models: [],
    });
    const rotated = await operationsAPI.keys.rotate('key-1');

    expect(managementMutationOutcome(issued)).toMatchObject({
      status: 'pending',
      command: { id: 'command-1' },
    });
    expect(managementMutationOutcome(rotated)).toMatchObject({
      status: 'pending',
      command: { id: 'command-1' },
    });
    expect(managementMutationOutcome(issued)).not.toHaveProperty('rawKey');
    expect(managementMutationOutcome(rotated)).not.toHaveProperty('rawKey');
  });

  it('sends the same explicit business reference and idempotency key for direct credit', async () => {
    const body = {
      amount: 5,
      reason: 'manual order verified',
      raise_keys: true,
      business_reference: 'manual-credit-1',
      idempotency_key: 'manual-credit-1',
    };
    await operationsAPI.users.recharge('user/id', body);
    expect(requestMock).toHaveBeenCalledWith('/litellmops/users/user%2Fid/recharge', {
      method: 'POST',
      data: body,
      skipErrorHandler: true,
    });

    await operationsAPI.recharges.reconcile('recharge/id');
    expect(requestMock).toHaveBeenLastCalledWith('/litellmops/recharges/recharge%2Fid/reconcile', {
      method: 'POST',
      data: undefined,
      skipErrorHandler: true,
    });
  });

  it('uses the protected management recovery contract without secret-bearing fields', async () => {
    const params = {
      page: 1,
      page_size: 20,
      user_id: 'user-1',
      status: 'result_unverified',
    } as const;
    await operationsAPI.management.commands(params);
    await operationsAPI.management.reconcile('command/id');
    await operationsAPI.management.resolve('command/id', {
      resolution: 'not_applied',
      reason: 'authoritative state checked',
      confirm_authoritative_state: true,
    });

    expect(requestMock).toHaveBeenNthCalledWith(1, '/litellmops/management/commands', {
      params,
      skipErrorHandler: true,
    });
    expect(requestMock).toHaveBeenNthCalledWith(
      2,
      '/litellmops/management/commands/command%2Fid/reconcile',
      {
        method: 'POST',
        data: undefined,
        skipErrorHandler: true,
      },
    );
    expect(requestMock).toHaveBeenNthCalledWith(
      3,
      '/litellmops/management/commands/command%2Fid/resolve',
      {
        method: 'POST',
        data: {
          resolution: 'not_applied',
          reason: 'authoritative state checked',
          confirm_authoritative_state: true,
        },
        skipErrorHandler: true,
      },
    );
  });

  it('surfaces uncertain create, update, block, and delete user responses for recovery', async () => {
    const pending = {
      code: 'management_reconcile_required',
      error: 'upstream result unknown',
      command: { id: 'command-user', status: 'result_unverified' },
    };
    requestMock.mockResolvedValue(pending);
    const input = { email: 'buyer@example.com', user_role: 'internal_user', models: [] };

    const results = await Promise.all([
      operationsAPI.users.create(input),
      operationsAPI.users.invite(input),
      operationsAPI.users.update('user-1', { max_budget: 5 }),
      operationsAPI.users.block('user-1'),
      operationsAPI.users.unblock('user-1'),
      operationsAPI.users.remove('user-1'),
    ]);

    for (const result of results) expect(managementMutationOutcome(result).status).toBe('pending');
  });

  it('uses the server sales state machine and exposes no browser connector import', async () => {
    await operationsAPI.sales.createOrder({
      channel: 'xianyu',
      shop: 'shop',
      external_order_id: 'order-1',
      adjustment_type: 'credit',
      user_email: 'buyer@example.com',
      paid_cny_fen: 1_000,
      payment_status: 'paid',
      source_trust: 'manual',
    });
    await operationsAPI.sales.reviewRefund('order/1', { reason: 'manual review required' });

    expect(requestMock).toHaveBeenNthCalledWith(1, '/litellmops/sales/orders', {
      method: 'POST',
      data: expect.objectContaining({ adjustment_type: 'credit', source_trust: 'manual' }),
      skipErrorHandler: true,
    });
    expect(requestMock).toHaveBeenNthCalledWith(
      2,
      '/litellmops/sales/orders/order%2F1/refund-review',
      {
        method: 'POST',
        data: { reason: 'manual review required' },
        skipErrorHandler: true,
      },
    );
    expect('import' in operationsAPI.sales).toBe(false);
  });

  it('carries the product CAS version and sends only supported list filters', async () => {
    const patch = {
      title: '10 USD credit',
      price_cny_fen: 1_000,
      credit_usd_micro: 10_000_000,
      raise_keys: true,
      enabled: true,
      auto_apply: false,
      version: 7,
    };
    await operationsAPI.sales.updateProduct('product/1', patch);
    await operationsAPI.sales.products({
      page: 1,
      page_size: 20,
      channel: 'xianyu',
      shop: 'shop',
      enabled: 'true',
    });
    await operationsAPI.sales.orders({
      page: 2,
      page_size: 20,
      status: 'received',
      external_order_id: 'external-1',
      user_email: 'buyer@example.com',
    });

    expect(requestMock).toHaveBeenNthCalledWith(1, '/litellmops/sales/products/product%2F1', {
      method: 'PATCH',
      data: patch,
      skipErrorHandler: true,
    });
    expect(requestMock).toHaveBeenNthCalledWith(2, '/litellmops/sales/products', {
      params: { page: 1, page_size: 20, channel: 'xianyu', shop: 'shop', enabled: 'true' },
      skipErrorHandler: true,
    });
    expect(requestMock).toHaveBeenNthCalledWith(3, '/litellmops/sales/orders', {
      params: {
        page: 2,
        page_size: 20,
        status: 'received',
        external_order_id: 'external-1',
        user_email: 'buyer@example.com',
      },
      skipErrorHandler: true,
    });
  });

  it('keeps organization and team access read-only in this release', async () => {
    await litellmopsAPI.listOrgs();
    await litellmopsAPI.getOrg('org/1');

    expect(requestMock).toHaveBeenNthCalledWith(1, '/litellmops/organizations', {
      method: 'GET',
      skipErrorHandler: true,
    });
    expect(requestMock).toHaveBeenNthCalledWith(2, '/litellmops/organizations/org%2F1', {
      method: 'GET',
      skipErrorHandler: true,
    });
    expect('createOrg' in litellmopsAPI).toBe(false);
    expect('updateOrg' in litellmopsAPI).toBe(false);
    expect('deleteOrg' in litellmopsAPI).toBe(false);
    expect('addOrgMember' in litellmopsAPI).toBe(false);
    expect('removeOrgMember' in litellmopsAPI).toBe(false);
    expect('createTeam' in litellmopsAPI).toBe(false);
    expect('deleteTeam' in litellmopsAPI).toBe(false);
    expect('updateTeam' in litellmopsAPI).toBe(false);
    expect('rechargeOrg' in litellmopsAPI).toBe(false);
    expect('rechargeTeam' in litellmopsAPI).toBe(false);
  });

  it('keeps gateway model access read-only in this release', () => {
    expect('block' in operationsAPI.gateway).toBe(false);
    expect('unblock' in operationsAPI.gateway).toBe(false);
  });
});
