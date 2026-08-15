import { render, screen } from '@testing-library/react';
import { App } from 'antd';
import { describe, expect, it, vi } from 'vitest';
import LanguageDetailDrawer from './LanguageDetailDrawer';

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
}));

vi.mock('./query', () => ({
  useLanguage: () => ({
    data: {
      id: 'language-1',
      name: 'en-US',
      remark: '',
      status: 'enabled',
      updatedAt: '2026-08-15T00:00:00Z',
      defines: [{ id: 'definition-id', group: 'menu', key: 'welcome', value: 'Welcome' }],
    },
    error: null,
    isError: false,
    isPending: false,
    refetch: vi.fn(),
  }),
}));

describe('language detail drawer', () => {
  it('renders the semantic group.key instead of the internal definition ID', async () => {
    render(
      <App>
        <LanguageDetailDrawer id="language-1" open onClose={vi.fn()} />
      </App>,
    );

    expect(await screen.findByText('menu.welcome')).toBeTruthy();
    expect(screen.queryByText('menu.definition-id')).toBeNull();
  });
});
