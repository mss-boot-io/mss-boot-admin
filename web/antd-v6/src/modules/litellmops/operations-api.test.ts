import { request } from '@umijs/max';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { operationsAPI } from './operations-api';
import { litellmopsAPI } from './api';

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
      data: undefined,
      skipErrorHandler: true,
    });
    expect(requestMock).toHaveBeenNthCalledWith(3, '/litellmops/keys/unsafe%2Fid', {
      method: 'DELETE',
      skipErrorHandler: true,
    });
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

  it('updates an organization through PATCH without inventing a team update route', async () => {
    const body = { organization_alias: 'Acme', max_budget: 100, models: ['gpt-5'] };
    await litellmopsAPI.updateOrg('org/1', body);

    expect(requestMock).toHaveBeenCalledWith('/litellmops/organizations/org%2F1', {
      method: 'PATCH',
      data: body,
      skipErrorHandler: true,
    });
    expect('updateTeam' in litellmopsAPI).toBe(false);
    expect('rechargeOrg' in litellmopsAPI).toBe(false);
    expect('rechargeTeam' in litellmopsAPI).toBe(false);
  });
});
