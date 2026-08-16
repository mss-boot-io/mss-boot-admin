import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import type { CurrentUser } from '@/shared/auth/types';
import HeaderNotice from './HeaderNotice';

const unread = vi.hoisted(() => vi.fn());

vi.mock('@umijs/max', () => ({
  Link: ({ children, to, ...props }: { children: ReactNode; to: string }) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
}));

vi.mock('@/modules/operations/api', () => ({
  operationsAPI: { notices: { unread } },
}));

function user(canRead: boolean): CurrentUser {
  return { id: 'user-1', permissions: { '/notice': canRead } };
}

function renderNotice(currentUser: CurrentUser) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <HeaderNotice user={currentUser} />
    </QueryClientProvider>,
  );
}

describe('header notice', () => {
  it('links authorized users to notification center and shows unread count', async () => {
    unread.mockResolvedValueOnce([{ id: 'notice-1' }, { id: 'notice-2' }]);

    renderNotice(user(true));

    const link = await screen.findByRole('link', { name: 'notice.header.label' });
    expect(link.getAttribute('href')).toBe('/notice');
    expect(await screen.findByText('2')).toBeTruthy();
  });

  it('does not expose the entry without notice permission', () => {
    renderNotice(user(false));
    expect(screen.queryByRole('link')).toBeNull();
  });
});
