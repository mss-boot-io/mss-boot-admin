import React, { useRef } from 'react';
import { useIntl } from '@@/exports';
import { ProColumns, ProFormInstance, ProTable } from '@ant-design/pro-components';
import { fieldIntl } from '@/util/fieldIntl';
import { getAppConfigsGroup, putAppConfigsGroup } from '@/services/admin/appConfig';
import { message } from 'antd';

const Email: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const formRef = useRef<ProFormInstance>();

  const columns: ProColumns<any>[] = [
    {
      title: fieldIntl(intl, 'smtpHost'),
      dataIndex: 'smtpHost',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
    },
    {
      title: fieldIntl(intl, 'smtpPort'),
      dataIndex: 'smtpPort',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
    },
    {
      title: fieldIntl(intl, 'username'),
      dataIndex: 'username',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
    },
    {
      title: fieldIntl(intl, 'password'),
      dataIndex: 'password',
      valueType: 'password',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
    },
  ];

  const onSubmit = async (params: Record<string, any>) => {
    await putAppConfigsGroup({ group: 'email' }, { data: params });
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
        request: async () => await getAppConfigsGroup({ group: 'email' }),
      }}
    />
  );
};

export default Email;
