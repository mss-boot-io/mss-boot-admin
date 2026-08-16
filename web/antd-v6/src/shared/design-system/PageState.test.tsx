import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { PageEmpty, PageError } from './PageState';

vi.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: ({ id }: { id: string }) =>
      ({ 'actions.retry': 'Retry', 'states.loadError': 'Unable to load' })[id] ?? id,
  }),
}));

describe('page states', () => {
  it('does not conflate confirmed empty and retryable error states', () => {
    const { rerender } = render(<PageEmpty description="没有数据" />);
    expect(screen.getByText('没有数据')).toBeTruthy();
    rerender(<PageError message="服务不可用" onRetry={vi.fn()} />);
    expect(screen.getByText('服务不可用')).toBeTruthy();
    expect(screen.getByText('Unable to load')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Retry/ })).toBeTruthy();
  });
});
