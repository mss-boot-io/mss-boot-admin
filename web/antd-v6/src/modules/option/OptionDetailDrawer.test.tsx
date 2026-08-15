import { render, screen } from '@testing-library/react';
import { App } from 'antd';
import { describe, expect, it, vi } from 'vitest';
import OptionDetailDrawer from './OptionDetailDrawer';

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
}));

vi.mock('./query', () => ({
  useOption: () => ({
    data: {
      id: 'option-1',
      category: 'system',
      displayName: 'Status',
      description: '',
      name: 'status',
      remark: '',
      status: 'enabled',
      version: 2,
      builtIn: false,
      updatedAt: '2026-08-15T00:00:00Z',
      items: [
        {
          id: 'item-id',
          key: 'enabled',
          label: 'Enabled',
          value: 'enabled',
          color: 'green',
          sort: 1,
          icon: '<script>alert(1)</script>',
          extra: { source: 'seed' },
        },
      ],
    },
    error: null,
    isError: false,
    isPending: false,
    refetch: vi.fn(),
  }),
}));

describe('option detail drawer', () => {
  it('renders dictionary metadata as inert text instead of dynamic icons or markup', async () => {
    render(
      <App>
        <OptionDetailDrawer id="option-1" open onClose={vi.fn()} />
      </App>,
    );
    expect(await screen.findAllByText('enabled')).toHaveLength(2);
    expect(screen.getByText(/<script>alert\(1\)<\/script>/)).toBeTruthy();
    expect(screen.getByText('{"source":"seed"}')).toBeTruthy();
  });
});
