import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { describe, expect, it, vi } from 'vitest';
import LocaleSwitcher from './LocaleSwitcher';

const localeRuntime = vi.hoisted(() => ({
  current: 'zh-CN',
  setLocale: vi.fn(),
}));

vi.mock('@umijs/max', () => ({
  getLocale: () => localeRuntime.current,
  setLocale: localeRuntime.setLocale,
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
}));

describe('LocaleSwitcher', () => {
  it('exposes an accessible trigger and delegates selection to the Umi locale runtime', async () => {
    render(
      <App>
        <LocaleSwitcher />
      </App>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'actions.switchLanguage' }));
    fireEvent.click(await screen.findByText('English'));

    await waitFor(() => expect(localeRuntime.setLocale).toHaveBeenCalledWith('en-US'));
  });
});
