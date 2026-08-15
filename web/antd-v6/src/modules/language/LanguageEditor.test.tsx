import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { LanguageDetail } from './contract';
import LanguageEditor from './LanguageEditor';

const languageQuery = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
const api = vi.hoisted(() => ({ create: vi.fn(), update: vi.fn() }));
const push = vi.hoisted(() => vi.fn());

vi.mock('@umijs/max', () => ({
  history: { push },
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
}));

vi.mock('./query', () => ({ useLanguage: () => languageQuery.current }));
vi.mock('./api', () => ({ languageAPI: api }));

function detail(): LanguageDetail {
  return {
    id: 'language-1',
    name: 'en-US',
    remark: '',
    status: 'enabled',
    updatedAt: '2026-08-15T00:00:00Z',
    defines: [],
  };
}

function renderEditor() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <App>
      <QueryClientProvider client={client}>
        <LanguageEditor id="language-1" mode="edit" />
      </QueryClientProvider>
    </App>,
  );
}

describe('language editor', () => {
  beforeEach(() => {
    languageQuery.current = {
      data: detail(),
      error: null,
      isError: false,
      isPending: false,
      refetch: vi.fn(async () => ({ data: detail() })),
    };
    api.create.mockReset();
    api.update.mockReset();
    api.update.mockRejectedValue({ response: { status: 409 }, message: 'conflict' });
    push.mockReset();
  });

  it('preserves the draft and exposes an explicit reload choice after a revision conflict', async () => {
    renderEditor();
    const name = await screen.findByLabelText('language.field.name');
    fireEvent.change(name, { target: { value: 'zh-CN' } });
    fireEvent.click(screen.getByRole('button', { name: /actions\.save/ }));

    await waitFor(() =>
      expect(api.update).toHaveBeenCalledWith(
        'language-1',
        expect.objectContaining({ name: 'zh-CN' }),
        '2026-08-15T00:00:00Z',
      ),
    );
    expect(await screen.findByText('language.conflict.title')).toBeTruthy();
    expect((screen.getByLabelText('language.field.name') as HTMLInputElement).value).toBe('zh-CN');
    expect(push).not.toHaveBeenCalled();
  });
});
