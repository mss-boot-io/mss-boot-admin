import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { afterEach, describe, expect, it, vi } from 'vitest';
import AdministrationTable from './AdministrationTable';

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
}));

vi.mock('@mss-admin-core/shared/design-system/ResponsiveEntityTable', () => ({
  default: () => <div data-testid="responsive-table" />,
}));

const query = {
  data: { current: 1, data: [], pageSize: 20, total: 0 },
  error: null,
  isError: false,
  isFetching: false,
  isPending: false,
  refetch: vi.fn(),
};

describe('administration search presentation', () => {
  afterEach(cleanup);

  it('reacts to an active collapsed default and keeps toolbar access while allowing expansion', async () => {
    const props = {
      columns: [],
      emptyText: 'No users',
      mobileColumnKeys: [],
      nameSearch: {
        component: 'input',
        field: 'name',
        label: 'Name filter',
        order: 10,
      },
      params: { current: 1, pageSize: 20 as const, status: 'all' as const },
      query: query as never,
      setParams: vi.fn(),
      toolbar: <button type="button">Create user</button>,
    };
    const view = render(
      <App>
        <AdministrationTable {...props} searchCollapsedByDefault={false} />
      </App>,
    );

    expect(screen.getByText('Name filter')).toBeTruthy();
    view.rerender(
      <App>
        <AdministrationTable {...props} searchCollapsedByDefault />
      </App>,
    );

    await waitFor(() => expect(screen.queryByText('Name filter')).toBeNull());
    expect(screen.getByRole('button', { name: 'Create user' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'actions.search' }));
    expect(screen.getByText('Name filter')).toBeTruthy();
  });
});
