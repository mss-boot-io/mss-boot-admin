import LockOutlined from '@ant-design/icons/LockOutlined';
import SaveOutlined from '@ant-design/icons/SaveOutlined';
import { getRequestErrorMessage } from '@mss-admin-core/shared/api/client';
import { PageError, PageLoading } from '@mss-admin-core/shared/design-system/PageState';
import { queryClient, queryKeys } from '@mss-admin-core/shared/query/client';
import { useQuery } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import { Alert, App, Button, Form, Input, InputNumber, Tag } from 'antd';
import { useEffect, useState } from 'react';
import { useAppConfigAccess } from './access';
import { appConfigAPI } from './api';
import { type EmailAppConfig, serializeEmailAppConfig } from './contracts';

export default function EmailPanel() {
  const intl = useIntl();
  const { message } = App.useApp();
  const { canReadSecrets, canWrite, canWriteSecrets } = useAppConfigAccess();
  const [form] = Form.useForm<EmailAppConfig>();
  const [saving, setSaving] = useState(false);
  const [operationError, setOperationError] = useState<string>();
  const query = useQuery({
    queryKey: queryKeys.appConfig('email'),
    queryFn: appConfigAPI.loadEmail,
    staleTime: 0,
  });

  useEffect(() => {
    if (query.data) form.setFieldsValue(query.data.values);
  }, [form, query.data]);

  const save = async (values: EmailAppConfig) => {
    if (!canWrite || saving) return;
    setSaving(true);
    setOperationError(undefined);
    try {
      await appConfigAPI.saveGroup('email', serializeEmailAppConfig(values, canWriteSecrets));
      form.setFieldValue('password', undefined);
      await queryClient.invalidateQueries({ queryKey: queryKeys.appConfig('email') });
      void message.success(intl.formatMessage({ id: 'pages.appConfig.saved' }));
    } catch (error) {
      setOperationError(getRequestErrorMessage(error));
    } finally {
      form.setFieldValue('password', undefined);
      setSaving(false);
    }
  };

  if (query.isPending && !query.data) return <PageLoading rows={5} />;
  if (query.isError || !query.data) {
    return (
      <PageError
        message={intl.formatMessage({ id: 'pages.appConfig.loadFailed' })}
        onRetry={() => void query.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }

  const passwordConfigured = query.data.configuredSecrets.has('password');
  return (
    <Form<EmailAppConfig>
      form={form}
      layout="vertical"
      disabled={!canWrite}
      onFinish={(values) => void save(values)}
    >
      <Alert
        className="mb-5"
        showIcon
        icon={<LockOutlined />}
        type="info"
        title={intl.formatMessage({ id: 'pages.appConfig.secretRotationTitle' })}
        description={intl.formatMessage({ id: 'pages.appConfig.secretRotationDescription' })}
      />
      {!canReadSecrets ? (
        <Alert
          className="mb-5"
          showIcon
          type="warning"
          title={intl.formatMessage({ id: 'pages.appConfig.secretReadRestricted' })}
        />
      ) : null}
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
        name="smtpHost"
        label={intl.formatMessage({ id: 'pages.appConfig.email.smtpHost' })}
        rules={[{ required: true }, { max: 255 }]}
      >
        <Input autoComplete="off" />
      </Form.Item>
      <Form.Item
        name="smtpPort"
        label={intl.formatMessage({ id: 'pages.appConfig.email.smtpPort' })}
        rules={[{ required: true }]}
      >
        <InputNumber controls={false} min={1} max={65_535} precision={0} className="w-full" />
      </Form.Item>
      <Form.Item
        name="username"
        label={intl.formatMessage({ id: 'pages.appConfig.email.username' })}
        rules={[{ required: true }, { max: 255 }]}
      >
        <Input autoComplete="off" />
      </Form.Item>
      <Form.Item
        name="password"
        label={
          <span>
            {intl.formatMessage({ id: 'pages.appConfig.email.password' })}{' '}
            {passwordConfigured ? (
              <Tag color="success">
                {intl.formatMessage({ id: 'pages.appConfig.secretConfigured' })}
              </Tag>
            ) : null}
          </span>
        }
        extra={intl.formatMessage({ id: 'pages.appConfig.secretLeaveBlank' })}
      >
        <Input.Password autoComplete="new-password" disabled={!canWriteSecrets} maxLength={512} />
      </Form.Item>
      {canWrite ? (
        <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
          {intl.formatMessage({ id: 'actions.save' })}
        </Button>
      ) : null}
    </Form>
  );
}
