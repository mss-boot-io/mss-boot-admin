import { describe, expect, it } from 'vitest';
import type { OptionDetail } from './contract';
import { presentOptionDetail, presentOptionSummary } from './presentation';

const intl = {
  formatMessage: ({ id }: { id: string }) => `translated:${id}`,
};

function statusOption(): OptionDetail {
  return {
    id: 'status',
    category: 'system',
    name: 'status',
    displayName: 'Status Options',
    description: 'Basic status options for all entities',
    remark: 'System status options',
    status: 'enabled',
    version: 1,
    builtIn: true,
    updatedAt: '2026-08-17T00:00:00Z',
    items: [
      {
        id: 'enabled',
        key: 'enabled',
        label: 'Enabled',
        value: 'enabled',
        color: 'green',
        sort: 1,
      },
    ],
  };
}

describe('built-in option presentation', () => {
  it('localizes untouched seed metadata and item labels', () => {
    const presented = presentOptionDetail(statusOption(), intl);

    expect(presented.displayName).toBe('translated:option.builtIn.status.displayName');
    expect(presented.description).toBe('translated:option.builtIn.status.description');
    expect(presented.remark).toBe('translated:option.builtIn.status.remark');
    expect(presented.items[0]?.label).toBe('translated:option.builtIn.status.item.enabled');
  });

  it('preserves customized values and non-built-in dictionaries verbatim', () => {
    const customized = { ...statusOption(), displayName: 'Operations status' };
    expect(presentOptionSummary(customized, intl).displayName).toBe('Operations status');

    const userOwned = { ...statusOption(), builtIn: false };
    expect(presentOptionDetail(userOwned, intl)).toBe(userOwned);
  });
});
