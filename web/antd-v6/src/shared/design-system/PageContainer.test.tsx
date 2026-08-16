import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { PageContainer } from './PageContainer';

vi.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children, title }: { children?: ReactNode; title?: ReactNode }) => (
    <main>
      <div>{title}</div>
      {children}
    </main>
  ),
}));

describe('semantic page container', () => {
  it('exposes the visible page title as the primary heading', () => {
    render(<PageContainer title="Users">content</PageContainer>);

    expect(screen.getByRole('heading', { level: 1, name: 'Users' })).toBeTruthy();
    expect(screen.getByText('content')).toBeTruthy();
  });
});
