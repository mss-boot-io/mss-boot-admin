import UploadOutlined from '@ant-design/icons/UploadOutlined';
import { useQuery } from '@tanstack/react-query';
import { useIntl, useModel } from '@umijs/max';
import type { UploadProps } from 'antd';
import { Alert, App, Avatar, Button, Form, Input, Space, Upload } from 'antd';
import { useEffect, useState } from 'react';
import { getRequestErrorMessage } from '@/shared/api/client';
import type { InitialState } from '@/shared/auth/types';
import { PageError, PageLoading } from '@/shared/design-system/PageState';
import { queryClient, queryKeys } from '@/shared/query/client';
import { useAppConfigAccess } from './access';
import { appConfigAPI } from './api';
import { type BaseAppConfig, serializeBaseAppConfig } from './contracts';
import { type InitialStateSetter, refreshApplicationRuntime } from './runtime';

export default function BasePanel() {
  const intl = useIntl();
  const { message } = App.useApp();
  const { canUpload, canWrite } = useAppConfigAccess();
  const { setInitialState } = useModel('@@initialState') as {
    initialState?: InitialState;
    setInitialState: InitialStateSetter;
  };
  const [form] = Form.useForm<BaseAppConfig>();
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [operationError, setOperationError] = useState<string>();
  const logo = Form.useWatch('websiteLogo', form);
  const query = useQuery({
    queryKey: queryKeys.appConfig('base'),
    queryFn: appConfigAPI.loadBase,
    staleTime: 0,
  });

  useEffect(() => {
    if (query.data) form.setFieldsValue(query.data);
  }, [form, query.data]);

  const uploadLogo: NonNullable<UploadProps['customRequest']> = async ({
    file,
    onError,
    onSuccess,
  }) => {
    if (!canUpload || !(file instanceof Blob)) {
      onError?.(new Error('Logo upload is unavailable'));
      return;
    }
    setUploading(true);
    setOperationError(undefined);
    try {
      const url = await appConfigAPI.uploadLogo(file);
      form.setFieldValue('websiteLogo', url);
      onSuccess?.({ url });
    } catch (error) {
      const normalized = error instanceof Error ? error : new Error('Logo upload failed');
      setOperationError(getRequestErrorMessage(error));
      onError?.(normalized);
    } finally {
      setUploading(false);
    }
  };

  const save = async (values: BaseAppConfig) => {
    if (!canWrite || saving) return;
    setSaving(true);
    setOperationError(undefined);
    try {
      await appConfigAPI.saveGroup('base', serializeBaseAppConfig(values));
      await queryClient.invalidateQueries({ queryKey: queryKeys.appConfig('base') });
      await refreshApplicationRuntime(setInitialState);
      void message.success(intl.formatMessage({ id: 'pages.appConfig.saved' }));
    } catch (error) {
      setOperationError(getRequestErrorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  if (query.isPending && !query.data) return <PageLoading rows={6} />;
  if (query.isError || !query.data) {
    return (
      <PageError
        message={intl.formatMessage({ id: 'pages.appConfig.loadFailed' })}
        onRetry={() => void query.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }

  return (
    <Form<BaseAppConfig>
      form={form}
      layout="vertical"
      disabled={!canWrite}
      requiredMark="optional"
      onFinish={(values) => void save(values)}
    >
      {operationError ? (
        <Alert
          className="mb-5"
          closable
          showIcon
          type="error"
          title={intl.formatMessage({ id: 'pages.appConfig.saveFailed' })}
          description={operationError}
          onClose={() => setOperationError(undefined)}
        />
      ) : null}
      <Form.Item
        name="websiteName"
        label={intl.formatMessage({ id: 'pages.appConfig.base.websiteName' })}
        rules={[{ required: true }, { max: 100 }]}
      >
        <Input />
      </Form.Item>
      <Form.Item
        name="websiteDescription"
        label={intl.formatMessage({ id: 'pages.appConfig.base.websiteDescription' })}
        rules={[{ max: 500 }]}
      >
        <Input.TextArea autoSize={{ minRows: 3, maxRows: 8 }} />
      </Form.Item>
      <Form.Item
        name="websiteLogo"
        label={intl.formatMessage({ id: 'pages.appConfig.base.websiteLogo' })}
        rules={[{ max: 2048 }]}
      >
        <Input />
      </Form.Item>
      <Space className="mb-6" align="center" wrap>
        <Avatar shape="square" size={72} src={logo} />
        {canUpload ? (
          <Upload
            accept="image/png,image/jpeg,image/webp"
            customRequest={uploadLogo}
            maxCount={1}
            showUploadList={false}
            beforeUpload={(file) => {
              if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
                void message.error(intl.formatMessage({ id: 'pages.appConfig.logoType' }));
                return Upload.LIST_IGNORE;
              }
              if (file.size > 2 * 1024 * 1024) {
                void message.error(intl.formatMessage({ id: 'pages.appConfig.logoSize' }));
                return Upload.LIST_IGNORE;
              }
              return true;
            }}
          >
            <Button icon={<UploadOutlined />} loading={uploading}>
              {intl.formatMessage({ id: 'pages.appConfig.uploadLogo' })}
            </Button>
          </Upload>
        ) : null}
      </Space>
      <Form.Item
        name="websiteRecordNumber"
        label={intl.formatMessage({ id: 'pages.appConfig.base.websiteRecordNumber' })}
        rules={[{ max: 100 }]}
      >
        <Input />
      </Form.Item>
      <Form.Item
        name="websiteCopyRight"
        label={intl.formatMessage({ id: 'pages.appConfig.base.websiteCopyRight' })}
        rules={[{ max: 255 }]}
      >
        <Input />
      </Form.Item>
      {canWrite ? (
        <Button type="primary" htmlType="submit" loading={saving}>
          {intl.formatMessage({ id: 'actions.save' })}
        </Button>
      ) : null}
    </Form>
  );
}
