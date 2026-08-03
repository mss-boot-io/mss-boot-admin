import { getAppConfigsGroup, putAppConfigsGroup } from '@/services/admin/appConfig';
import { ProColumns, ProFormInstance, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { message } from 'antd';
import React, { useRef } from 'react';

const Storage: React.FC = () => {
  const intl = useIntl();

  const fromRef = useRef<ProFormInstance>();
  const [s3, setS3] = React.useState(false);

  const columns: ProColumns<any>[] = [
    {
      title: 'type',
      dataIndex: 'type',
      valueEnum: {
        local: {
          text: 'local',
          color: 'red',
          status: 'local',
        },
        s3: {
          text: 's3',
          color: 'blue',
          status: 's3',
        },
      },
    },
    {
      title: 'endpoint',
      dataIndex: 'endpoint',
      tooltip: '本地存储访问路径前缀',
    },
    {
      title: 'maxSize',
      dataIndex: 'maxSize',
      tooltip: '最大文件大小(字节)，默认10MB',
      valueType: 'digit',
      fieldProps: {
        placeholder: '10485760 (10MB)',
      },
    },
    {
      title: 'allowedTypes',
      dataIndex: 'allowedTypes',
      tooltip: '允许的文件类型，逗号分隔',
      valueType: 'textarea',
      fieldProps: {
        placeholder: 'image/jpeg,image/png,image/gif,application/pdf',
        rows: 3,
      },
    },
    {
      title: 's3Provider',
      dataIndex: 's3Provider',
      hideInForm: !s3,
      valueEnum: {
        s3: {
          text: 's3(aws)',
          status: 's3',
        },
        oss: {
          text: 'oss(aliyun)',
          status: 'oss',
        },
        minio: {
          text: 'minio',
          status: 'minio',
        },
        gcs: {
          text: 'gcs(google)',
          status: 'gcs',
        },
        oos: {
          text: 'oos(ctyun)',
          status: 'oos',
        },
        kodo: {
          text: 'kodo(qiniu)',
          status: 'kodo',
        },
        cos: {
          text: 'cos(tencent)',
          status: 'cos',
        },
        obs: {
          text: 'obs(huawei)',
          status: 'obs',
        },
        bos: {
          text: 'bos(baidu)',
          status: 'bos',
        },
        ks3: {
          text: 'ks3(kingsoft)',
          status: 'ks3',
        },
      },
    },
    {
      title: 's3Endpoint',
      dataIndex: 's3Endpoint',
      hideInForm: !s3,
    },
    {
      title: 's3Region',
      dataIndex: 's3Region',
      hideInForm: !s3,
    },
    {
      title: 's3Bucket',
      dataIndex: 's3Bucket',
      hideInForm: !s3,
    },
    {
      title: 's3AccessKeyID',
      dataIndex: 's3AccessKeyID',
      hideInForm: !s3,
    },
    {
      title: 's3SecretAccessKey',
      dataIndex: 's3SecretAccessKey',
      hideInForm: !s3,
      valueType: 'password',
    },
    {
      title: 's3SigningMethod',
      dataIndex: 's3SigningMethod',
      hideInForm: !s3,
    },
  ];

  const onSubmit = async (params: Record<string, any>) => {
    params.type = s3 ? 's3' : 'local';
    await putAppConfigsGroup({ group: 'storage' }, { data: params });
    message.success(
      intl.formatMessage({ id: 'pages.message.edit.success', defaultMessage: 'Update Success!' }),
    );
  };

  return (
    <ProTable<any>
      type="form"
      formRef={fromRef}
      columns={columns}
      onSubmit={onSubmit}
      form={{
        request: async () => {
          const res = await getAppConfigsGroup({ group: 'storage' });
          // setLocal(res.type?.value === 'local');
          setS3(res.type === 's3');
          if (!res.type) {
            // setLocal(true);
            res.type = 'local';
          }
          // setMinio(res.s3Provider?.value === 'minio');
          return res;
        },
        onValuesChange: (values) => {
          if (values.type) {
            // setLocal(values.type.value === 'local');
            setS3(values.type === 's3');
          }
          if (values.s3Provider) {
            // setMinio(values.s3Provider.value === 'minio');
          }
        },
      }}
    />
  );
};

export default Storage;
