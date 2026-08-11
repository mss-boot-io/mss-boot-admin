import { getAppConfigsGroup, putAppConfigsGroup } from '@/services/admin/appConfig';
import { ProColumns, ProFormInstance, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { message } from 'antd';
import React, { useRef } from 'react';
import { useAppConfigAccess } from '../useAppConfigAccess';

type StorageAdmissionConfig = {
  allowedTypes?: string;
  maxSize?: number | string;
};

const Storage: React.FC = () => {
  const intl = useIntl();
  const { canWrite } = useAppConfigAccess();
  const formRef = useRef<ProFormInstance>();

  const columns: ProColumns<StorageAdmissionConfig>[] = [
    {
      title: 'maxSize',
      dataIndex: 'maxSize',
      tooltip: 'Maximum object size in bytes; default 10485760, hard ceiling 104857600',
      valueType: 'digit',
      fieldProps: {
        min: 1,
        max: 104857600,
        precision: 0,
        placeholder: '10485760 (10 MiB)',
      },
    },
    {
      title: 'allowedTypes',
      dataIndex: 'allowedTypes',
      tooltip: 'Comma-separated MIME media types or type/* wildcards',
      valueType: 'textarea',
      fieldProps: {
        placeholder: 'image/jpeg,image/png,image/*,application/pdf',
        rows: 3,
      },
    },
  ];

  const onSubmit = async (params: StorageAdmissionConfig) => {
    if (!canWrite) return;
    await putAppConfigsGroup(
      { group: 'storage' },
      {
        data: {
          allowedTypes: params.allowedTypes,
          maxSize: params.maxSize,
        },
      },
    );
    message.success(
      intl.formatMessage({ id: 'pages.message.edit.success', defaultMessage: 'Update Success!' }),
    );
  };

  return (
    <ProTable<StorageAdmissionConfig>
      type="form"
      formRef={formRef}
      columns={columns}
      onSubmit={canWrite ? onSubmit : undefined}
      form={{
        readonly: !canWrite,
        submitter: canWrite ? undefined : false,
        request: async () => {
          const res = await getAppConfigsGroup({ group: 'storage' });
          return {
            allowedTypes: res.allowedTypes,
            maxSize: res.maxSize,
          };
        },
      }}
    />
  );
};

export default Storage;
