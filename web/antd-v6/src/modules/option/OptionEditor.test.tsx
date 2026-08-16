import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { OptionDetail } from './contract';
import OptionEditor from './OptionEditor';

const optionQuery = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
const api = vi.hoisted(() => ({ create: vi.fn(), update: vi.fn() }));
const push = vi.hoisted(() => vi.fn());

vi.mock('@umijs/max', () => ({
  history: { push },
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
}));

vi.mock('./query', () => ({ useOption: () => optionQuery.current }));
vi.mock('./api', () => ({ optionAPI: api }));

function detail(): OptionDetail {
  return {
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
    items: [],
  };
}

function renderEditor() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <App>
      <QueryClientProvider client={client}>
        <OptionEditor id="option-1" mode="edit" />
      </QueryClientProvider>
    </App>,
  );
}

describe('option editor', () => {
  beforeEach(() => {
    optionQuery.current = {
      data: detail(),
      error: null,
      isError: false,
      isPending: false,
      refetch: vi.fn(async () => ({ data: detail() })),
    };
    api.create.mockReset();
    api.update.mockReset();
    api.update.mockRejectedValue({ response: { status: 412 }, message: 'conflict' });
    push.mockReset();
  });

  it('preserves the draft and exact base after a revision conflict', async () => {
    renderEditor();
    const name = await screen.findByLabelText('option.field.name');
    fireEvent.change(name, { target: { value: 'changed-name' } });
    fireEvent.click(screen.getByRole('button', { name: /actions\.save/ }));

    await waitFor(() =>
      expect(api.update).toHaveBeenCalledWith(
        'option-1',
        expect.objectContaining({ name: 'changed-name' }),
        expect.objectContaining({ id: 'option-1', version: 2 }),
      ),
    );
    expect(await screen.findByText('option.conflict.title')).toBeTruthy();
    expect((screen.getByLabelText('option.field.name') as HTMLInputElement).value).toBe(
      'changed-name',
    );
    expect(push).not.toHaveBeenCalled();
  });
});
