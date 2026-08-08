import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import HeaderSearch from './index';

const React = require('react');

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: ({ defaultMessage, id }: { defaultMessage?: string; id: string }) =>
      defaultMessage || id,
  }),
}));

describe('HeaderSearch', () => {
  it('opens from a named button, focuses the input, and returns focus on Escape', async () => {
    render(<HeaderSearch options={[]} />);

    const trigger = screen.getByRole('button', { name: 'Open menu search' });
    const input = screen.getByRole('combobox', { name: 'Search menus' });
    expect(input.getAttribute('tabindex')).toBe('-1');

    fireEvent.click(trigger);
    await waitFor(() => expect(document.activeElement).toBe(input));
    expect(trigger.getAttribute('aria-expanded')).toBe('true');
    expect(input.getAttribute('tabindex')).toBe('0');

    fireEvent.mouseDown(trigger);
    fireEvent.click(trigger);
    expect(trigger.getAttribute('aria-expanded')).toBe('false');
    expect(document.activeElement).toBe(trigger);

    fireEvent.click(trigger);
    await waitFor(() => expect(document.activeElement).toBe(input));

    fireEvent.keyDown(input, { key: 'Escape' });
    expect(document.activeElement).toBe(trigger);
    expect(trigger.getAttribute('aria-expanded')).toBe('false');
    expect(input.getAttribute('tabindex')).toBe('-1');
  });
});
