import { getAppConfigsGroup, putAppConfigsGroup } from '@/services/admin/appConfig';
import { ProColumns, ProFormInstance, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { message, Upload } from 'antd';
import React, { useRef, useState } from 'react';
import { request } from '@@/exports';
import { PlusOutlined } from '@ant-design/icons';

const Base: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const formRef = useRef<ProFormInstance>();
  const [logo, setLogo] = useState<string>('');

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'pages.appConfig.base.websiteName' }),
      dataIndex: 'websiteName',
      initialValue: 'mss-boot-admin',
      formItemProps: {
        rules: [
          {
            required: true,
            message: intl.formatMessage({ id: 'pages.form.required' }),
          },
        ],
      },
    },
    {
      title: intl.formatMessage({ id: 'pages.appConfig.base.websiteDescription' }),
      dataIndex: 'websiteDescription',
      initialValue: '快速开发http/grpc服务的框架，帮助您快速搭建单体服务或微服务系统',
      valueType: 'textarea',
    },
    {
      title: intl.formatMessage({ id: 'pages.appConfig.base.websiteLogo' }),
      dataIndex: 'websiteLogo',
      valueType: 'avatar',
      renderFormItem: () => {
        return (
          <Upload
            listType="picture-circle"
            className="avatar-uploader"
            showUploadList={false}
            customRequest={async ({ file }) => {
              const formData = new FormData();

              formData.append('file', file);
              const res = await request('/admin/api/storage/upload', {
                method: 'POST',
                data: formData,
              });
              setLogo(res);
            }}
          >
            {logo ? <img src={logo} alt="avatar" style={{ width: '100%' }} /> : <PlusOutlined />}
          </Upload>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'pages.appConfig.base.websiteRecordNumber' }),
      dataIndex: 'websiteRecordNumber',
    },
    {
      title: intl.formatMessage({ id: 'pages.appConfig.base.websiteCopyRight' }),
      initialValue: '开源组织mss-boot-io出品',
      dataIndex: 'websiteCopyRight',
    },
  ];

  const onSubmit = async (params: Record<string, any>) => {
    params.websiteLogo = logo;
    await putAppConfigsGroup({ group: 'base' }, { data: params });
    message.success(
      intl.formatMessage({ id: 'pages.message.edit.success', defaultMessage: 'Update Success!' }),
    );
  };

  return (
    <ProTable<any>
      type="form"
      formRef={formRef}
      columns={columns}
      onSubmit={onSubmit}
      form={{
        request: async () => {
          const res = await getAppConfigsGroup({ group: 'base' });
          setLogo(res?.websiteLogo || 'https://docs.mss-boot-io.top/favicon.ico');
          return res;
        },
      }}
    />
  );
};

export default Base;
