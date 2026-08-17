import { describe, expect, it } from 'vitest';
import {
  OptionContractError,
  type OptionDetail,
  optionRevisionETag,
  parseOptionDetail,
  parseOptionPage,
  serializeOptionWrite,
} from './contract';

const summary = {
  id: 'option-1',
  category: 'system',
  displayName: 'Status',
  name: 'status',
  remark: '',
  status: 'enabled',
  version: 3,
  builtIn: false,
  updatedAt: '2026-08-15T00:00:00Z',
} as const;

const detail: OptionDetail = {
  ...summary,
  description: 'Status values',
  items: [
    {
      id: 'item-1',
      key: 'enabled',
      label: 'Enabled',
      value: 'enabled',
      color: 'green',
      sort: 1,
      icon: 'check',
      extra: { source: 'seed' },
    },
  ],
};

describe('option contract', () => {
  it('parses bounded summaries independently from full item payloads', () => {
    expect(
      parseOptionPage(
        { data: [summary], total: 1, current: 1, pageSize: 20 },
        { current: 1, pageSize: 20 },
      ),
    ).toMatchObject({ total: 1, data: [{ name: 'status', version: 3 }] });
    expect(parseOptionDetail(detail).items[0]).toMatchObject({ id: 'item-1', key: 'enabled' });
  });

  it('rejects malformed pagination, duplicate identity, and unsafe opaque metadata', () => {
    expect(() =>
      parseOptionPage(
        { data: [summary], total: 1, current: 2, pageSize: 20 },
        { current: 1, pageSize: 20 },
      ),
    ).toThrow(OptionContractError);
    expect(() =>
      parseOptionDetail({
        ...detail,
        items: [
          { id: 'one', key: 'same', label: 'One', value: '1', color: '', sort: 1 },
          { id: 'two', key: 'same', label: 'Two', value: '2', color: '', sort: 2 },
        ],
      }),
    ).toThrow(/unique/);
    expect(() =>
      parseOptionDetail({
        ...detail,
        items: [
          {
            id: 'one',
            key: 'one',
            label: 'One',
            value: '1',
            color: '',
            sort: 1,
            extra: JSON.parse('{"__proto__":{"polluted":true}}'),
          },
        ],
      }),
    ).toThrow(/extra key/);
  });

  it('owns create identities and preserves server opaque metadata on exact updates', () => {
    const values = {
      id: 'client-record',
      category: ' system ',
      displayName: ' Status ',
      description: ' Values ',
      name: ' status ',
      remark: ' note ',
      status: 'enabled' as const,
      items: [
        {
          id: 'item-1',
          key: ' enabled ',
          label: ' Enabled ',
          value: ' enabled ',
          color: ' green ',
          sort: 1,
          icon: ' check ',
        },
      ],
    };
    const create = serializeOptionWrite(values);
    expect(create).toMatchObject({
      category: 'system',
      displayName: 'Status',
      description: 'Values',
      name: 'status',
      remark: 'note',
      status: 'enabled',
    });
    expect(create).not.toHaveProperty('id');
    expect(create.items[0]).not.toHaveProperty('id');

    const update = serializeOptionWrite(values, { base: detail });
    expect(update).not.toHaveProperty('expectedVersion');
    expect(update.items[0]).toMatchObject({ id: 'item-1', extra: { source: 'seed' } });
  });

  it('omits protected built-in identity fields while retaining editable metadata', () => {
    const builtIn = { ...detail, builtIn: true };
    const payload = serializeOptionWrite(
      {
        category: 'changed',
        displayName: 'Visible',
        description: '',
        name: 'changed',
        status: 'disabled',
        items: [],
      },
      { base: builtIn },
    );
    expect(payload).toEqual({
      displayName: 'Visible',
      description: '',
      remark: '',
      items: [],
    });
  });

  it('derives the canonical strong ETag from the parsed server revision', () => {
    expect(optionRevisionETag(summary)).toBe('"option-option-1-v3"');
  });
});
