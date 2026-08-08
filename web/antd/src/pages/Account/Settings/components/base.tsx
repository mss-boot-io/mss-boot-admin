import {
  ProColumns,
  ProFormDependency,
  ProFormInstance,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components';
import { message, Upload } from 'antd';
import React, { useEffect, useRef } from 'react';
import { LoadingOutlined, PlusOutlined } from '@ant-design/icons';
import { getUserUserInfo, putUserUserInfo } from '@/services/admin/user';
import { request, useIntl, useModel } from '@umijs/max';
import { useRequest } from 'ahooks';
import { city } from '../geographic/city';
import { province } from '../geographic/province';
import { fieldIntl } from '@/util/fieldIntl';

type ProfileFormValues = API.UpdateUserInfoRequest & {
  username?: string;
};

type SelectOption = {
  label: string;
  value: string;
};

type AvatarUploadFieldProps = {
  loading: boolean;
  onChange?: (avatar: string) => void;
  value?: string;
};

const citiesByProvince = city as Record<string, Array<{ id: string; name: string }>>;

export const getProvinceOptions = (): SelectOption[] =>
  province.map((item) => ({ label: item.name, value: item.id }));

export const getCityOptions = (provinceID?: string): SelectOption[] =>
  (provinceID ? citiesByProvince[provinceID] ?? [] : []).map((item) => ({
    label: item.name,
    value: item.id,
  }));

export const clearProfileCity = (form?: Pick<ProFormInstance, 'setFieldsValue'>) =>
  form?.setFieldsValue({ city: undefined });

export const buildProfileUpdateRequest = (
  values: ProfileFormValues,
): API.UpdateUserInfoRequest => ({
  address: values.address,
  avatar: values.avatar,
  city: values.city,
  country: values.country,
  email: values.email,
  group: values.group,
  name: values.name,
  phone: values.phone,
  profile: values.profile,
  province: values.province,
  signature: values.signature,
  tags: values.tags,
  title: values.title,
});

export const AvatarUploadField: React.FC<AvatarUploadFieldProps> = ({
  loading,
  onChange,
  value,
}) => (
  <Upload
    name="avatar"
    listType="picture-circle"
    className="avatar-uploader"
    showUploadList={false}
    customRequest={async ({ file, onError, onSuccess }) => {
      if (!(file instanceof Blob)) {
        onError?.(new Error('Invalid avatar file'));
        return;
      }

      try {
        const formData = new FormData();
        formData.append('file', file);
        const response = await request<{ avatar: string }>('/admin/api/user/avatar', {
          method: 'POST',
          data: formData,
        });
        onChange?.(response.avatar);
        onSuccess?.(response);
      } catch (error) {
        onError?.(error instanceof Error ? error : new Error('Avatar upload failed'));
      }
    }}
  >
    {value ? (
      <img src={value} alt="avatar" style={{ width: '100%' }} />
    ) : loading ? (
      <LoadingOutlined />
    ) : (
      <PlusOutlined />
    )}
  </Upload>
);

const BaseView: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();
  const { initialState, loading: initialStateLoading, setInitialState } =
    useModel('@@initialState');

  const formRef = useRef<ProFormInstance>();

  const initialUser = initialState?.currentUser;
  const { data: fetchedUser, loading: userInfoLoading } = useRequest(getUserUserInfo, {
    // Wait for Umi's initial state before falling back so a slow initial-state
    // load cannot race and duplicate the current-user request.
    ready: !initialStateLoading && !initialUser,
  });
  const currentUser = initialUser ?? fetchedUser;
  const loading = initialStateLoading || userInfoLoading;

  useEffect(() => {
    if (!currentUser) return;
    formRef.current?.setFieldsValue({
      ...currentUser,
      country: currentUser.country || 'China',
    });
  }, [currentUser]);

  const columns: ProColumns<ProfileFormValues>[] = [
    {
      title: fieldIntl(intl, 'username'),
      dataIndex: 'username',
      valueType: 'text',
      width: 'md',
      fieldProps: {
        disabled: true,
      },
      formItemProps: {
        rules: [{ required: true }, { min: 3 }, { max: 20 }, { pattern: /^[a-zA-Z0-9_]+$/ }],
      },
    },
    {
      title: fieldIntl(intl, 'avatar'),
      dataIndex: 'avatar',
      valueType: 'avatar',
      renderFormItem: () => <AvatarUploadField loading={loading} />,
    },
    {
      title: fieldIntl(intl, 'email'),
      dataIndex: 'email',
      valueType: 'text',
      width: 'md',
      formItemProps: {
        rules: [
          {
            required: true,
          },
        ],
      },
    },
    // {
    //   title: '新密码',
    //   dataIndex: 'password',
    //   valueType: 'password',
    //   width: 'md',
    //   formItemProps: {
    //     rules: [
    //       { min: 8 },
    //       { max: 20 },
    //       {
    //         pattern: /[a-zA-Z]/,
    //         message: intl.formatMessage({
    //           id: 'pages.message.password.rule.pattern.letters',
    //           defaultMessage: 'The password must contain letters',
    //         }),
    //       },
    //       {
    //         pattern: /[0-9]/,
    //         message: intl.formatMessage({
    //           id: 'pages.message.password.rule.pattern.numbers',
    //           defaultMessage: 'The password must contain numbers',
    //         }),
    //       },
    //     ],
    //   },
    // },
    // {
    //   title: '确认密码',
    //   dataIndex: 'confirm',
    //   valueType: 'password',
    //   width: 'md',
    //   formItemProps: {
    //     rules: [
    //       { min: 8 },
    //       { max: 20 },
    //       {
    //         pattern: /[a-zA-Z]/,
    //         message: intl.formatMessage({
    //           id: 'pages.message.password.rule.pattern.letters',
    //           defaultMessage: 'The password must contain letters',
    //         }),
    //       },
    //       {
    //         pattern: /[0-9]/,
    //         message: intl.formatMessage({
    //           id: 'pages.message.password.rule.pattern.numbers',
    //           defaultMessage: 'The password must contain numbers',
    //         }),
    //       },
    //     ],
    //   },
    // },
    {
      title: fieldIntl(intl, 'nickname'),
      dataIndex: 'name',
      valueType: 'text',
      width: 'md',
      formItemProps: {
        rules: [
          {
            required: true,
          },
        ],
      },
    },
    {
      title: fieldIntl(intl, 'profile'),
      dataIndex: 'profile',
      valueType: 'textarea',
      width: 'md',
      formItemProps: {
        rules: [
          {
            required: true,
          },
        ],
      },
    },
    {
      // title: '国家/地区',
      title: fieldIntl(intl, 'country'),
      dataIndex: 'country',
      valueType: 'select',
      width: 'md',
      valueEnum: {
        China: intl.formatMessage({
          id: 'pages.account.settings.country.china',
          defaultMessage: 'China',
        }),
      },
    },
    {
      title: fieldIntl(intl, 'province'),
      dataIndex: 'province',
      valueType: 'select',
      width: 'md',
      formItemProps: {
        rules: [
          {
            required: true,
          },
        ],
      },
      renderFormItem: () => {
        return (
          <>
            <ProFormSelect
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'pages.account.settings.province.required',
                    defaultMessage: 'Please select your province',
                  }),
                },
              ]}
              width="sm"
              name="province"
              onChange={() => {
                clearProfileCity(formRef.current);
              }}
              request={async () => getProvinceOptions()}
            />
            <ProFormDependency name={['province']}>
              {({ province: selectedProvince }: { province?: string }) => {
                return (
                  <ProFormSelect
                    params={{
                      province: selectedProvince,
                    }}
                    name="city"
                    width="sm"
                    rules={[
                      {
                        required: true,
                        message: intl.formatMessage({
                          id: 'pages.account.settings.city.required',
                          defaultMessage: 'Please select your city',
                        }),
                      },
                    ]}
                    disabled={!selectedProvince}
                    request={async () => getCityOptions(selectedProvince)}
                  />
                );
              }}
            </ProFormDependency>
          </>
        );
      },
    },
    {
      title: fieldIntl(intl, 'address'),
      dataIndex: 'address',
      valueType: 'text',
      width: 'md',
      formItemProps: {
        rules: [
          {
            required: true,
          },
        ],
      },
    },
    {
      title: fieldIntl(intl, 'phone'),
      dataIndex: 'phone',
      valueType: 'text',
      width: 'md',
      formItemProps: {
        rules: [
          {
            required: true,
          },
        ],
      },
    },
  ];

  const handleFinish = async (values: ProfileFormValues) => {
    const data = buildProfileUpdateRequest(values);
    await putUserUserInfo(data);
    setInitialState((state) => ({
      ...state,
      currentUser: {
        ...currentUser,
        ...data,
      },
    }));
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
  };
  return (
    <ProTable<ProfileFormValues>
      type="form"
      formRef={formRef}
      columns={columns}
      onSubmit={handleFinish}
      form={{
        initialValues: {
          ...currentUser,
          country: currentUser?.country || 'China',
        },
        layout: 'vertical',
        requiredMark: false,
        submitter: {
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.account.settings.profile.submit',
              defaultMessage: 'Update profile',
            }),
          },
          render: (_, dom) => dom[1],
        },
      }}
    />
  );
};

export default BaseView;
