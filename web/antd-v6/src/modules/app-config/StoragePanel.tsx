import SaveOutlined from '@ant-design/icons/SaveOutlined';
import { getRequestErrorMessage } from '@mss-admin-core/shared/api/client';
import { PageError, PageLoading } from '@mss-admin-core/shared/design-system/PageState';
import { queryClient, queryKeys } from '@mss-admin-core/shared/query/client';
import { useQuery } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import { Alert, App, Button, Form, Input, InputNumber } from 'antd';
import { useEffect, useState } from 'react';
import { useAppConfigAccess } from './access';
import { appConfigAPI } from './api';
import { type StorageAppConfig, serializeStorageAppConfig } from './contracts';

export default function StoragePanel() {
  const intl = useIntl();
  const { message } = App.useApp();
  const { canWrite } = useAppConfigAccess();
  const [form] = Form.useForm<StorageAppConfig>();
  const [saving, setSaving] = useState(false);
  const [operationError, setOperationError] = useState<string>();
  const query = useQuery({
    queryKey: queryKeys.appConfig('storage'),
    queryFn: appConfigAPI.loadStorage,
    staleTime: 0,
  });

  useEffect(() => {
    if (query.data) form.setFieldsValue(query.data);
  }, [form, query.data]);

  const save = async (values: StorageAppConfig) => {
    if (!canWrite || saving) return;
    setSaving(true);
    setOperationError(undefined);
    try {
      await appConfigAPI.saveGroup('storage', serializeStorageAppConfig(values));
      await queryClient.invalidateQueries({ queryKey: queryKeys.appConfig('storage') });
      void message.success(intl.formatMessage({ id: 'pages.appConfig.saved' }));
    } catch (error) {
      setOperationError(getRequestErrorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  if (query.isPending && !query.data) return <PageLoading rows={4} />;
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
    <Form<StorageAppConfig>
      form={form}
      layout="vertical"
      disabled={!canWrite}
      onFinish={(values) => void save(values)}
    >
      <Alert
        className="mb-5"
        showIcon
        type="info"
        title={intl.formatMessage({ id: 'pages.appConfig.storage.admissionOnlyTitle' })}
        description={intl.formatMessage({
          id: 'pages.appConfig.storage.admissionOnlyDescription',
        })}
      />
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
        name="maxSize"
        label={intl.formatMessage({ id: 'pages.appConfig.storage.maxSize' })}
        extra={intl.formatMessage({ id: 'pages.appConfig.storage.maxSizeHelp' })}
        rules={[{ required: true }]}
      >
        <InputNumber className="w-full" controls={false} max={104_857_600} min={1} precision={0} />
      </Form.Item>
      <Form.Item
        name="allowedTypes"
        label={intl.formatMessage({ id: 'pages.appConfig.storage.allowedTypes' })}
        extra={intl.formatMessage({ id: 'pages.appConfig.storage.allowedTypesHelp' })}
      >
        <Input.TextArea rows={4} placeholder="image/jpeg,image/png,image/*,application/pdf" />
      </Form.Item>
      {canWrite ? (
        <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
          {intl.formatMessage({ id: 'actions.save' })}
        </Button>
      ) : null}
    </Form>
  );
}
