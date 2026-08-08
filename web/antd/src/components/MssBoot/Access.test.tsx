import { render, screen } from '@testing-library/react';
import { useModel } from '@umijs/max';
import { Access } from './Access';

const React = require('react');

jest.mock('@umijs/max', () => ({
  useModel: jest.fn(),
}));

const mockUseModel = useModel as jest.Mock;

const renderAccess = (permission?: string) =>
  render(
    <Access permission={permission} fallback={<span>denied</span>}>
      <span>allowed</span>
    </Access>,
  );

describe('Access', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('uses the explicit permission prop when a React key is also present', () => {
    mockUseModel.mockReturnValue({
      initialState: { currentUser: { permissions: { '/users/edit': true } } },
    });

    render(
      <Access key="/users/edit" permission="/users/edit" fallback={<span>denied</span>}>
        <span>allowed</span>
      </Access>,
    );

    expect(screen.getByText('allowed')).toBeTruthy();
  });

  it('allows root users without requiring a named permission', () => {
    mockUseModel.mockReturnValue({
      initialState: { currentUser: { role: { root: true } } },
    });

    renderAccess('/users/delete');

    expect(screen.getByText('allowed')).toBeTruthy();
  });

  it('supports permission maps without a hasOwnProperty method', () => {
    const permissions = Object.create(null) as Record<string, boolean>;
    permissions['/users/edit'] = true;
    mockUseModel.mockReturnValue({ initialState: { currentUser: { permissions } } });

    renderAccess('/users/edit');

    expect(screen.getByText('allowed')).toBeTruthy();
  });

  it('does not trust an overridden permission hasOwnProperty method', () => {
    const permissions = { hasOwnProperty: jest.fn(() => true) };
    mockUseModel.mockReturnValue({ initialState: { currentUser: { permissions } } });

    renderAccess('/users/delete');

    expect(screen.getByText('denied')).toBeTruthy();
    expect(permissions.hasOwnProperty).not.toHaveBeenCalled();
  });

  it('does not authorize a false or non-boolean permission value', () => {
    mockUseModel.mockReturnValue({
      initialState: {
        currentUser: { permissions: { '/users/edit': false, '/users/delete': 'component' } },
      },
    });

    renderAccess('/users/edit');
    expect(screen.getByText('denied')).toBeTruthy();
  });

  it('keeps root-only controls exclusive to root identities', () => {
    mockUseModel.mockReturnValue({
      initialState: { currentUser: { permissions: { '/users/create': true } } },
    });
    render(
      <Access rootOnly permission="/users/create" fallback={<span>denied</span>}>
        <span>allowed</span>
      </Access>,
    );
    expect(screen.getByText('denied')).toBeTruthy();
  });
});
