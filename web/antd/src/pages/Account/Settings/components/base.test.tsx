import * as React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import enUSPages from '@/locales/en-US/pages';
import zhCNPages from '@/locales/zh-CN/pages';
import { putUserUserInfo } from '@/services/admin/user';
import BaseView, {
  AvatarUploadField,
  buildProfileUpdateRequest,
  clearProfileCity,
  getCityOptions,
  getProvinceOptions,
} from './base';

const mockSetInitialState = jest.fn();
const mockRequest = jest.fn();

const mockCurrentUser: API.User = {
  address: 'West Lake',
  avatar: 'https://example.com/avatar.png',
  city: '110100',
  country: 'China',
  email: 'user@example.com',
  id: 'user-1',
  name: 'Example User',
  phone: '13800000000',
  profile: 'Profile text',
  province: '110000',
  username: 'example_user',
};

jest.mock('@umijs/max', () => ({
  request: (...args: unknown[]) => mockRequest(...args),
  useIntl: () => ({
    formatMessage: ({ id, defaultMessage }: { id: string; defaultMessage?: string }) =>
      defaultMessage || id,
  }),
  useModel: () => ({
    initialState: { currentUser: mockCurrentUser },
    loading: false,
    setInitialState: mockSetInitialState,
  }),
}));

jest.mock('ahooks', () => ({
  useRequest: () => ({ data: undefined, loading: false }),
}));

jest.mock('@/services/admin/user', () => ({
  getUserUserInfo: jest.fn(),
  putUserUserInfo: jest.fn(),
}));

jest.mock('antd', () => {
  const actual = jest.requireActual('antd');
  return {
    ...actual,
    message: { ...actual.message, success: jest.fn() },
  };
});

describe('account profile form', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (putUserUserInfo as jest.Mock).mockResolvedValue({});
  });

  it('uses string province and city options and safely handles an unknown province', () => {
    expect(getProvinceOptions()).toContainEqual(expect.objectContaining({ value: '110000' }));
    expect(getCityOptions('110000')).toContainEqual(
      expect.objectContaining({ value: '110100' }),
    );
    expect(getCityOptions('unknown')).toEqual([]);
    expect(getCityOptions()).toEqual([]);
  });

  it('clears the stale city when the province changes', () => {
    const setFieldsValue = jest.fn();

    clearProfileCity({ setFieldsValue });

    expect(setFieldsValue).toHaveBeenCalledWith({ city: undefined });
  });

  it('builds the backend profile contract without leaking form-only fields', () => {
    const request = buildProfileUpdateRequest({
      ...mockCurrentUser,
      province: '110000',
      city: '110100',
    });

    expect(request).toEqual(
      expect.objectContaining({
        avatar: mockCurrentUser.avatar,
        city: '110100',
        province: '110000',
      }),
    );
    expect(request).not.toHaveProperty('username');
    expect(request).not.toHaveProperty('email');
    expect(typeof request.province).toBe('string');
    expect(typeof request.city).toBe('string');
  });

  it('absorbs the form value instead of passing an invalid value prop to Upload', () => {
    const consoleError = jest.spyOn(console, 'error').mockImplementation(() => undefined);

    render(<AvatarUploadField loading={false} value={mockCurrentUser.avatar} />);

    expect(screen.getByRole('img', { name: 'avatar' }).getAttribute('src')).toBe(
      mockCurrentUser.avatar,
    );
    expect(
      consoleError.mock.calls.some((args) => args.join(' ').includes('value is not a valid prop')),
    ).toBe(false);
    consoleError.mockRestore();
  });

  it('submits province and city as strings', async () => {
    render(<BaseView />);
    expect((screen.getByDisplayValue('example_user') as HTMLInputElement).disabled).toBe(true);
    const submitButton = screen.getByRole('button', { name: 'Update profile' });
    expect(submitButton).toBeTruthy();

    fireEvent.click(submitButton!);

    await waitFor(() => {
      expect(putUserUserInfo).toHaveBeenCalledWith(
        expect.objectContaining({ province: '110000', city: '110100' }),
      );
    });
    const payload = (putUserUserInfo as jest.Mock).mock.calls[0][0];
    expect(typeof payload.province).toBe('string');
    expect(typeof payload.city).toBe('string');
    expect(payload).not.toHaveProperty('username');
  });

  it('keeps the profile field locale synchronized in Chinese and English', () => {
    expect(enUSPages['pages.fields.profile']).toBe('profile');
    expect(zhCNPages['pages.fields.profile']).toBe('个人简介');
    expect(enUSPages['pages.fields.nickname']).toBe('display name');
    expect(enUSPages['pages.fields.country']).toBe('country or region');
    expect(enUSPages['pages.fields.province']).toBe('province');
    expect(enUSPages['pages.fields.address']).toBe('address');
    expect(zhCNPages['pages.fields.nickname']).toBe('显示名称');
    expect(zhCNPages['pages.fields.country']).toBe('国家或地区');
    expect(zhCNPages['pages.fields.province']).toBe('省份');
    expect(zhCNPages['pages.fields.address']).toBe('详细地址');
  });
});
