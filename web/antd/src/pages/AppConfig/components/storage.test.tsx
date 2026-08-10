import { getAppConfigsGroup, putAppConfigsGroup } from '@/services/admin/appConfig';
import { ProTable } from '@ant-design/pro-components';
import { render } from '@testing-library/react';
import { useModel } from '@umijs/max';
import Storage from './storage';

const React = require('react');

jest.mock('@ant-design/pro-components', () => ({ ProTable: jest.fn(() => null) }));
jest.mock('@umijs/max', () => ({
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
  useModel: jest.fn(),
}));
jest.mock('@/services/admin/appConfig', () => ({
  getAppConfigsGroup: jest.fn(),
  putAppConfigsGroup: jest.fn(),
}));
jest.mock('antd', () => ({ message: { success: jest.fn() } }));

const mockProTable = ProTable as unknown as jest.Mock;

describe('AppConfig storage admission policy', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (getAppConfigsGroup as jest.Mock).mockResolvedValue({
      allowedTypes: 'image/png,image/*',
      maxSize: '10485760',
      s3Endpoint: 'https://legacy.invalid',
      s3SecretAccessKey: 'must-not-enter-the-form',
      type: 's3',
    });
    (useModel as jest.Mock).mockReturnValue({
      initialState: {
        currentUser: { permissions: { '/app-config/control': true } },
      },
    });
  });

  it('renders, reads, and writes only maxSize and allowedTypes', async () => {
    render(<Storage />);
    const props = mockProTable.mock.calls[0][0];

    expect(props.columns.map((column: { dataIndex?: string }) => column.dataIndex)).toEqual([
      'maxSize',
      'allowedTypes',
    ]);
    expect(props.columns[0].fieldProps).toMatchObject({
      min: 1,
      max: 104857600,
      precision: 0,
    });
    await expect(props.form.request()).resolves.toEqual({
      allowedTypes: 'image/png,image/*',
      maxSize: '10485760',
    });

    await props.onSubmit({
      allowedTypes: 'image/png',
      maxSize: 2048,
      s3SecretAccessKey: 'must-not-submit',
      type: 's3',
    });
    expect(putAppConfigsGroup).toHaveBeenCalledWith(
      { group: 'storage' },
      { data: { allowedTypes: 'image/png', maxSize: 2048 } },
    );
  });

  it('keeps the admission policy read-only without AppConfig control', () => {
    (useModel as jest.Mock).mockReturnValue({
      initialState: { currentUser: { permissions: {} } },
    });

    render(<Storage />);
    const props = mockProTable.mock.calls[0][0];
    expect(props.onSubmit).toBeUndefined();
    expect(props.form).toMatchObject({ readonly: true, submitter: false });
  });
});
