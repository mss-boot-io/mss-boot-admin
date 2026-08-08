import React, { useRef } from 'react';
import { useIntl } from '@@/exports';
import { ProColumns, ProFormInstance, ProTable } from '@ant-design/pro-components';
import { fieldIntl } from '@/util/fieldIntl';
import { getAppConfigsGroup, putAppConfigsGroup } from '@/services/admin/appConfig';
import { message } from 'antd';
import {
  omitAppConfigSecrets,
  prepareAppConfigSecretPayload,
  useAppConfigAccess,
} from '../useAppConfigAccess';

const Email: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();
  const { canReadSecrets, canWrite, canWriteSecrets } = useAppConfigAccess();

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
      fieldProps: {
        disabled: !canWriteSecrets,
      },
      formItemProps: () => {
        return {
          rules: [{ required: canReadSecrets && canWriteSecrets }],
        };
      },
    },
  ];

  const onSubmit = async (params: Record<string, any>) => {
    if (!canWrite) return;
    const data = prepareAppConfigSecretPayload('email', params, {
      canReadSecrets,
      canWriteSecrets,
    });
    await putAppConfigsGroup({ group: 'email' }, { data });
    message.success(
      intl.formatMessage({ id: 'pages.message.edit.success', defaultMessage: 'Update Success!' }),
    );
  };

  return (
    <ProTable<any>
      type="form"
      formRef={formRef}
      columns={columns}
      onSubmit={canWrite ? onSubmit : undefined}
      form={{
        readonly: !canWrite,
        submitter: canWrite ? undefined : false,
        request: async () => {
          const config = await getAppConfigsGroup({ group: 'email' });
          return canReadSecrets ? config : omitAppConfigSecrets('email', config);
        },
      }}
    />
  );
};

export default Email;
