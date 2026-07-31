import {
  ProColumns,
  ProFormDependency,
  ProFormInstance,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components';
import { message, Upload } from 'antd';
import React, { useRef, useState } from 'react';
import { LoadingOutlined, PlusOutlined } from '@ant-design/icons';
import { getUserUserInfo, putUserUserInfo } from '@/services/admin/user';
import { request, useIntl } from '@umijs/max';
import { useRequest } from 'ahooks';
import { city } from '../geographic/city';
import { province } from '../geographic/province';
import { fieldIntl } from '@/util/fieldIntl';

const BaseView: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const formRef = useRef<ProFormInstance>();
  const [avatar, setAvatar] = useState<string>('');

  const getProvince = () => {
    return province.map((item) => {
      return {
        label: item.name,
        value: item.id,
      };
    });
  };

  const getCity = (key: string) => {
    // @ts-ignore
    return city[key].map((item) => {
      return {
        label: item.name,
        value: item.id,
      };
    });
  };

  const { data: currentUser, loading } = useRequest(async () => {
    const res = await getUserUserInfo();
    if (res) {
      const img = res.avatar;
      if (img) {
        setAvatar(img);
      }
      return res;
    }
    return {};
  });

  const columns: ProColumns<any>[] = [
    {
      title: fieldIntl(intl, 'username'),
      dataIndex: 'username',
      valueType: 'text',
      width: 'md',
      formItemProps: {
        rules: [{ required: true }, { min: 3 }, { max: 20 }, { pattern: /^[a-zA-Z0-9_]+$/ }],
      },
    },
    {
      title: fieldIntl(intl, 'avatar'),
      dataIndex: 'avatar',
      valueType: 'avatar',
      renderFormItem: () => {
        return (
          <Upload
            name="avatar"
            listType="picture-circle"
            className="avatar-uploader"
            showUploadList={false}
            customRequest={async ({ file }) => {
              const formData = new FormData();

              formData.append('file', file);
              const res = await request('/admin/api/user/avatar', {
                method: 'POST',
                data: formData,
              });
              setAvatar(res.avatar);
            }}
          >
            {avatar ? (
              <img src={avatar} alt="avatar" style={{ width: '100%' }} />
            ) : loading ? (
              <LoadingOutlined />
            ) : (
              <PlusOutlined />
            )}
          </Upload>
        );
      },
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
        China: '中国',
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
                  message: '请输入您的所在省!',
                },
              ]}
              width="sm"
              fieldProps={{
                labelInValue: true,
              }}
              name="province"
              onChange={() => {
                formRef.current?.setFieldsValue({
                  city: undefined,
                });
              }}
              // @ts-ignore
              request={getProvince}
            />
            <ProFormDependency name={['province']}>
              {({ province }) => {
                return (
                  <ProFormSelect
                    params={{
                      key: province?.value,
                    }}
                    name="city"
                    width="sm"
                    rules={[
                      {
                        required: true,
                        message: '请输入您的所在城市!',
                      },
                    ]}
                    disabled={!province}
                    request={async () => {
                      if (!province?.key) {
                        if (currentUser?.province) {
                          return getCity(currentUser?.province);
                        }
                        return [];
                      }
                      return getCity(province.key);
                    }}
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

  const handleFinish = async () => {
    const data = formRef.current?.getFieldsValue();
    data.avatar = avatar;
    await putUserUserInfo(data);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
  };
  return (
    <ProTable
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
            submitText: '更新基本信息',
          },
          render: (_, dom) => dom[1],
        },
      }}
    />
  );
};

export default BaseView;
