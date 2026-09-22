import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { CurrentUser } from '../auth/types';
import { defineWorkplaceContributions } from './contract';
import { WorkplaceContributions } from './WorkplaceContributions';

const session = vi.hoisted(() => ({ user: undefined as CurrentUser | undefined }));
vi.mock('@umijs/max', () => ({
  useModel: () => ({ initialState: { currentUser: session.user } }),
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
}));
afterEach(() => {
  cleanup();
  session.user = undefined;
});

describe('workplace business contributions', () => {
  it('does not mount without identity or the declared permission', () => {
    const widget = vi.fn(() => <p>Private business content</p>);
    const entries = defineWorkplaceContributions([
      { id: 'example', permission: 'example:read', component: widget },
    ]);
    const view = render(<WorkplaceContributions contributions={entries} />);
    expect(widget).not.toHaveBeenCalled();
    session.user = { id: 'reader', permissions: { 'example:read': false } };
    view.rerender(<WorkplaceContributions contributions={entries} />);
    expect(widget).not.toHaveBeenCalled();
    session.user = { id: 'reader', permissions: { 'example:read': true } };
    view.rerender(<WorkplaceContributions contributions={entries} />);
    expect(screen.getByText('Private business content')).toBeTruthy();
    session.user = { id: 'reader', permissions: { 'example:read': false } };
    view.rerender(<WorkplaceContributions contributions={entries} />);
    expect(screen.queryByText('Private business content')).toBeNull();
  });
  it('isolates render failure and does not reveal thrown details', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    session.user = { id: 'reader', permissions: { 'example:read': true } };
    const Failed = () => {
      throw new Error('private upstream detail');
    };
    const entries = defineWorkplaceContributions([
      { id: 'failed', permission: 'example:read', component: Failed },
      { id: 'working', permission: 'example:read', component: () => <p>Working contribution</p> },
    ]);
    render(<WorkplaceContributions contributions={entries} />);
    expect(screen.getByText('pages.workplace.businessUnavailable')).toBeTruthy();
    expect(screen.getByText('Working contribution')).toBeTruthy();
    expect(screen.queryByText('private upstream detail')).toBeNull();
  });
  it('rejects duplicate, unbounded and permission-free registration', () => {
    const entry = { id: 'example', permission: 'example:read', component: () => null };
    expect(() => defineWorkplaceContributions([entry, entry])).toThrow();
    expect(() => defineWorkplaceContributions([{ ...entry, permission: '' }])).toThrow();
    expect(() =>
      defineWorkplaceContributions(
        Array.from({ length: 13 }, (_, i) => ({ ...entry, id: `item-${i}` })),
      ),
    ).toThrow();
    const registered = defineWorkplaceContributions([entry]);
    expect(Object.isFrozen(registered)).toBe(true);
    expect(Object.isFrozen(registered[0])).toBe(true);
  });
});
