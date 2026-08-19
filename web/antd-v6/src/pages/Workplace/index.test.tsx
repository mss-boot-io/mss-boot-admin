import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import WorkplacePage from './index';

const state = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));

vi.mock('@umijs/max', () => ({
  Link: ({ children, to }: { children: ReactNode; to: string }) => <a href={to}>{children}</a>,
  useIntl: () => ({
    formatMessage: ({ id }: { id: string }, values?: Record<string, string>) =>
      values?.name ? `${id}:${values.name}` : id,
  }),
  useModel: () => ({ initialState: state.current }),
}));

vi.mock('@ant-design/pro-components', () => ({
  ProCard: ({ children, title }: { children: ReactNode; title?: ReactNode }) => (
    <section>
      {title}
      {children}
    </section>
  ),
}));

vi.mock('@mss-admin-core/shared/design-system/PageContainer', () => ({
  PageContainer: ({ children, title }: { children: ReactNode; title: ReactNode }) => (
    <main>
      <h1>{title}</h1>
      {children}
    </main>
  ),
}));

vi.mock('@mss-admin-core/modules/monitor/MonitorOverview', () => ({
  default: () => <div>monitor</div>,
}));

describe('workplace quick links', () => {
  beforeEach(() => {
    state.current = {
      currentUser: {
        username: 'alice',
        permissions: { '/users': true, '/departments': true, '/log': false },
        role: { name: 'Operator', root: false },
      },
    };
  });

  it('keeps personal links and exposes only authorized management shortcuts', () => {
    render(<WorkplacePage />);

    expect(screen.getByRole('link', { name: /menu\.account-center/ })).toBeTruthy();
    expect(screen.getByRole('link', { name: /menu\.account-settings/ })).toBeTruthy();
    expect(screen.getByRole('link', { name: /menu\.users/ })).toBeTruthy();
    expect(screen.getByRole('link', { name: /menu\.departments/ })).toBeTruthy();
    expect(screen.queryByRole('link', { name: /menu\.system-log/ })).toBeNull();
    expect(screen.queryByRole('link', { name: /menu\.role/ })).toBeNull();
  });
});
