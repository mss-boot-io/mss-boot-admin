import { render } from '@testing-library/react';
import { ProTable } from '@ant-design/pro-components';
import { useModel } from '@umijs/max';
import { getAppConfigsGroup, putAppConfigsGroup } from '@/services/admin/appConfig';
import Email from './email';

const React = require('react');

jest.mock('@ant-design/pro-components', () => ({ ProTable: jest.fn(() => null) }));
jest.mock('@umijs/max', () => ({ useModel: jest.fn() }));
jest.mock('@@/exports', () => ({
  useModel: jest.fn(),
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
}));
jest.mock('@/services/admin/appConfig', () => ({
  getAppConfigsGroup: jest.fn(),
  putAppConfigsGroup: jest.fn(),
}));
jest.mock('antd', () => ({ message: { success: jest.fn() } }));

const mockProTable = ProTable as unknown as jest.Mock;

describe('AppConfig email secret permissions', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (getAppConfigsGroup as jest.Mock).mockResolvedValue({
      username: 'admin',
      password: 'stored-secret',
    });
  });

  const renderWithPermissions = (permissions: Record<string, unknown>) => {
    (useModel as jest.Mock).mockReturnValue({ initialState: { currentUser: { permissions } } });
    render(<Email />);
    return mockProTable.mock.calls[0][0];
  };

  it('redacts and strips the password when secret permissions are absent', async () => {
    const props = renderWithPermissions({ '/app-config/control': true });
    const passwordColumn = props.columns.find(
      (column: { dataIndex?: string }) => column.dataIndex === 'password',
    );

    expect(passwordColumn.fieldProps).toMatchObject({ disabled: true });
    await expect(props.form.request()).resolves.toEqual({ username: 'admin' });
    await props.onSubmit({ username: 'updated', password: 'must-not-leak' });
    expect(putAppConfigsGroup).toHaveBeenCalledWith(
      { group: 'email' },
      { data: { username: 'updated' } },
    );
  });

  it('allows a blind secret rotation with write permission but still hides the stored value', async () => {
    const props = renderWithPermissions({
      '/app-config/control': true,
      '/app-config/secrets/write': true,
    });
    const passwordColumn = props.columns.find(
      (column: { dataIndex?: string }) => column.dataIndex === 'password',
    );

    expect(passwordColumn.fieldProps).toMatchObject({ disabled: false });
    await expect(props.form.request()).resolves.toEqual({ username: 'admin' });
    await props.onSubmit({ username: 'updated', password: 'rotated-secret' });
    expect(putAppConfigsGroup).toHaveBeenCalledWith(
      { group: 'email' },
      { data: { username: 'updated', password: 'rotated-secret' } },
    );
  });

  it.each([
    ['empty', ''],
    ['whitespace-only', '   '],
    ['undefined', undefined],
    ['null', null],
  ])('does not clear the stored password during a blind %s submission', async (_, password) => {
    const props = renderWithPermissions({
      '/app-config/control': true,
      '/app-config/secrets/write': true,
    });

    await props.onSubmit({ username: 'updated', password });

    expect(putAppConfigsGroup).toHaveBeenCalledWith(
      { group: 'email' },
      { data: { username: 'updated' } },
    );
  });
});
