import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { history } from '@umijs/max';
import { requestAuthorizedMenu } from '@/utils/requestAuthorizedMenu';
import AuthorizedMenuSearch, {
  buildAuthorizedMenuSearchItems,
  filterAuthorizedMenuSearchItems,
} from './index';

const React = require('react');

jest.mock('@/utils/requestAuthorizedMenu', () => ({
  requestAuthorizedMenu: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  history: { push: jest.fn() },
  useIntl: () => ({
    messages: {
      'menu.system': 'System',
      'menu.origination.user': 'Users',
    },
    formatMessage: ({ defaultMessage, id }: { defaultMessage?: string; id: string }) => {
      const messages: Record<string, string> = {
        'menu.system': 'System',
        'menu.users': 'Users',
        'menu.origination.user': 'Users',
      };
      return messages[id] || defaultMessage || id;
    },
  }),
}));

const mockedRequestAuthorizedMenu = requestAuthorizedMenu as jest.Mock;
const mockedHistoryPush = history.push as jest.Mock;

describe('authorized menu search items', () => {
  it('includes only unique, visible, navigable menu nodes', () => {
    const items = buildAuthorizedMenuSearchItems(
      [
        {
          name: 'system',
          path: '/system',
          type: 'DIRECTORY',
          children: [
            { name: 'users', path: '/users', type: 'MENU' },
            { name: 'create', path: '/users/create', type: 'COMPONENT' },
            { name: 'duplicate', path: '/users', type: 'MENU' },
          ],
        },
        { name: 'hidden', path: '/hidden', type: 'MENU', hideInMenu: true },
        { name: 'disabled', path: '/disabled', type: 'MENU', status: 'disabled' },
        { name: 'external', path: 'https://example.com', type: 'MENU' },
        { name: 'parameterized', path: '/users/:id', type: 'MENU' },
      ],
      (name) => ({ system: 'System', users: 'Users' }[name] || name),
    );

    expect(items).toEqual([
      expect.objectContaining({
        label: 'System / Users',
        path: '/users',
      }),
    ]);
    expect(filterAuthorizedMenuSearchItems(items, 'USERS')).toHaveLength(1);
    expect(filterAuthorizedMenuSearchItems(items, '/users')).toHaveLength(1);
    expect(filterAuthorizedMenuSearchItems(items, 'missing')).toEqual([]);
  });
});

describe('AuthorizedMenuSearch', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedRequestAuthorizedMenu.mockResolvedValue([
      {
        name: 'system',
        path: '/system',
        type: 'DIRECTORY',
        children: [{ name: '/users', path: '/users', type: 'MENU' }],
      },
    ]);
  });

  it('loads the identity-scoped menu and navigates to the selected result', async () => {
    const identity = { role: { root: false }, permissions: { '/users': true } };
    render(<AuthorizedMenuSearch identity={identity} permissionRefreshVersion={6} />);

    await waitFor(() =>
      expect(mockedRequestAuthorizedMenu).toHaveBeenCalledWith(identity, 6),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Open menu search' }));
    const input = screen.getByRole('combobox', { name: 'Search menus' });
    fireEvent.change(input, {
      target: { value: 'users' },
    });
    fireEvent.click(await screen.findByText('System / Users'));

    await waitFor(() => expect(mockedHistoryPush).toHaveBeenCalledWith('/users'));
  });

  it('shows a localized empty result state', async () => {
    render(<AuthorizedMenuSearch identity={{ role: { root: false } }} />);

    await waitFor(() => expect(mockedRequestAuthorizedMenu).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: 'Open menu search' }));
    fireEvent.change(screen.getByRole('combobox', { name: 'Search menus' }), {
      target: { value: 'missing' },
    });

    expect(await screen.findByText('No accessible menus found.')).toBeTruthy();
  });

  it('retries a failed shared request without retaining stale menu results', async () => {
    mockedRequestAuthorizedMenu
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce([{ name: 'users', path: '/users', type: 'MENU' }]);
    render(<AuthorizedMenuSearch identity={{ role: { root: false } }} />);

    fireEvent.click(screen.getByRole('button', { name: 'Open menu search' }));
    fireEvent.change(screen.getByRole('combobox', { name: 'Search menus' }), {
      target: { value: 'users' },
    });
    expect(await screen.findByText('Unable to load menus.')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

    expect(await screen.findByText('Users')).toBeTruthy();
    expect(mockedRequestAuthorizedMenu).toHaveBeenCalledTimes(2);
  });
});
