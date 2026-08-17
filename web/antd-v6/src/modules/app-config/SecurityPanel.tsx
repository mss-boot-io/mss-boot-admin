import LockOutlined from '@ant-design/icons/LockOutlined';
import SaveOutlined from '@ant-design/icons/SaveOutlined';
import { useQuery } from '@tanstack/react-query';
import { useIntl, useModel } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Form,
  Input,
  Row,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { useEffect, useState } from 'react';
import { getRequestErrorMessage } from '@/shared/api/client';
import type { InitialState } from '@/shared/auth/types';
import { PageError, PageLoading } from '@/shared/design-system/PageState';
import { queryClient, queryKeys } from '@/shared/query/client';
import { useAppConfigAccess } from './access';
import { appConfigAPI } from './api';
import {
  type SecurityAppConfig,
  type SecurityAppConfigSecretKey,
  serializeSecurityAppConfig,
} from './contracts';
import { type InitialStateSetter, refreshApplicationRuntime } from './runtime';

interface SecretFieldProps {
  configured: boolean;
  disabled: boolean;
  label: string;
  name: SecurityAppConfigSecretKey;
  required?: boolean;
}

function SecretField({ configured, disabled, label, name, required }: SecretFieldProps) {
  const intl = useIntl();
  return (
    <Form.Item
      name={name}
      label={
        <span>
          {label}{' '}
          {configured ? (
            <Tag color="success">
              {intl.formatMessage({ id: 'pages.appConfig.secretConfigured' })}
            </Tag>
          ) : null}
        </span>
      }
      extra={intl.formatMessage({ id: 'pages.appConfig.secretLeaveBlank' })}
      rules={[{ required }]}
    >
      <Input.Password autoComplete="new-password" disabled={disabled} maxLength={512} />
    </Form.Item>
  );
}

export default function SecurityPanel() {
  const intl = useIntl();
  const { message } = App.useApp();
  const { canReadSecrets, canWrite, canWriteSecrets } = useAppConfigAccess();
  const { setInitialState } = useModel('@@initialState') as {
    initialState?: InitialState;
    setInitialState: InitialStateSetter;
  };
  const [form] = Form.useForm<SecurityAppConfig>();
  const [saving, setSaving] = useState(false);
  const [operationError, setOperationError] = useState<string>();
  const githubEnabled = Form.useWatch('githubEnabled', form);
  const larkEnabled = Form.useWatch('larkEnabled', form);
  const query = useQuery({
    queryKey: queryKeys.appConfig('security'),
    queryFn: appConfigAPI.loadSecurity,
    staleTime: 0,
  });

  useEffect(() => {
    if (query.data) form.setFieldsValue(query.data.values);
  }, [form, query.data]);

  const save = async (values: SecurityAppConfig) => {
    if (!canWrite || saving) return;
    setSaving(true);
    setOperationError(undefined);
    try {
      await appConfigAPI.saveGroup('security', serializeSecurityAppConfig(values, canWriteSecrets));
      form.setFieldsValue({
        githubBrowserSessionClientSecret: undefined,
        larkBrowserSessionAppSecret: undefined,
      });
      await queryClient.invalidateQueries({ queryKey: queryKeys.appConfig('security') });
      await refreshApplicationRuntime(setInitialState);
      void message.success(intl.formatMessage({ id: 'pages.appConfig.saved' }));
    } catch (error) {
      setOperationError(getRequestErrorMessage(error));
    } finally {
      form.setFieldsValue({
        githubBrowserSessionClientSecret: undefined,
        larkBrowserSessionAppSecret: undefined,
      });
      setSaving(false);
    }
  };

  if (query.isPending && !query.data) return <PageLoading rows={10} />;
  if (query.isError || !query.data) {
    return (
      <PageError
        message={intl.formatMessage({ id: 'pages.appConfig.loadFailed' })}
        onRetry={() => void query.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }

  const configured = query.data.configuredSecrets;
  const urlRule = { type: 'url' as const };
  return (
    <Form<SecurityAppConfig>
      form={form}
      layout="vertical"
      disabled={!canWrite}
      onFinish={(values) => void save(values)}
    >
      <Space orientation="vertical" size="large" className="w-full">
        <Alert
          showIcon
          icon={<LockOutlined />}
          type="info"
          title={intl.formatMessage({ id: 'pages.appConfig.secretRotationTitle' })}
          description={intl.formatMessage({ id: 'pages.appConfig.secretRotationDescription' })}
        />
        {!canReadSecrets ? (
          <Alert
            showIcon
            type="warning"
            title={intl.formatMessage({ id: 'pages.appConfig.secretReadRestricted' })}
          />
        ) : null}
        {operationError ? (
          <Alert
            closable
            showIcon
            type="error"
            title={intl.formatMessage({ id: 'pages.appConfig.saveFailed' })}
            description={operationError}
            onClose={() => setOperationError(undefined)}
          />
        ) : null}

        <Card title={intl.formatMessage({ id: 'pages.appConfig.security.general' })} size="small">
          <Row gutter={[24, 0]}>
            <Col xs={24} md={12} xl={8}>
              <Form.Item
                name="registerEnabled"
                label={intl.formatMessage({ id: 'pages.appConfig.security.registerEnabled' })}
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
            <Col xs={24} md={12} xl={8}>
              <Form.Item
                name="emailEnabled"
                label={intl.formatMessage({ id: 'pages.appConfig.security.emailEnabled' })}
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
            <Col xs={24} md={12} xl={8}>
              <Form.Item
                name="phoneEnabled"
                label={intl.formatMessage({ id: 'pages.appConfig.security.phoneEnabled' })}
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
            <Col xs={24} md={12} xl={8}>
              <Form.Item
                name="githubEnabled"
                label={intl.formatMessage({ id: 'pages.appConfig.security.githubEnabled' })}
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
            <Col xs={24} md={12} xl={8}>
              <Form.Item
                name="larkEnabled"
                label={intl.formatMessage({ id: 'pages.appConfig.security.larkEnabled' })}
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
          </Row>
        </Card>

        <Card
          title={intl.formatMessage(
            { id: 'pages.appConfig.security.browserOAuthTitle' },
            { provider: 'GitHub' },
          )}
          size="small"
          extra={
            <Tag color="blue">{intl.formatMessage({ id: 'pages.appConfig.recommended' })}</Tag>
          }
        >
          <Typography.Paragraph type="secondary">
            {intl.formatMessage({ id: 'pages.appConfig.security.browserOAuthDescription' })}
          </Typography.Paragraph>
          <Row gutter={[24, 0]}>
            <Col xs={24} lg={12}>
              <Form.Item
                name="githubBrowserSessionClientId"
                label={intl.formatMessage({ id: 'pages.appConfig.security.clientId' })}
                rules={[{ required: Boolean(githubEnabled) }, { max: 255 }]}
              >
                <Input autoComplete="off" />
              </Form.Item>
            </Col>
            <Col xs={24} lg={12}>
              <SecretField
                name="githubBrowserSessionClientSecret"
                label={intl.formatMessage({ id: 'pages.appConfig.security.clientSecret' })}
                configured={configured.has('githubBrowserSessionClientSecret')}
                disabled={!canWriteSecrets}
                required={
                  Boolean(githubEnabled) &&
                  canWriteSecrets &&
                  !configured.has('githubBrowserSessionClientSecret')
                }
              />
            </Col>
            <Col xs={24} lg={16}>
              <Form.Item
                name="githubBrowserSessionRedirectURI"
                label={intl.formatMessage({ id: 'pages.appConfig.security.redirectUri' })}
                rules={[{ required: Boolean(githubEnabled) }, urlRule, { max: 2048 }]}
              >
                <Input />
              </Form.Item>
            </Col>
            <Col xs={24} lg={8}>
              <Form.Item
                name="githubBrowserSessionScope"
                label={intl.formatMessage({ id: 'pages.appConfig.security.scope' })}
                rules={[{ max: 500 }]}
              >
                <Input placeholder="read:user,user:email" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item
                name="githubAllowGroup"
                label={intl.formatMessage({ id: 'pages.appConfig.security.githubAllowGroup' })}
                rules={[{ max: 1000 }]}
              >
                <Input />
              </Form.Item>
            </Col>
          </Row>
        </Card>

        <Card
          title={intl.formatMessage(
            { id: 'pages.appConfig.security.browserOAuthTitle' },
            { provider: 'Lark' },
          )}
          size="small"
          extra={<Tag color="blue">V6</Tag>}
        >
          <Typography.Paragraph type="secondary">
            {intl.formatMessage({ id: 'pages.appConfig.security.browserOAuthDescription' })}
          </Typography.Paragraph>
          <Row gutter={[24, 0]}>
            <Col xs={24} lg={12}>
              <Form.Item
                name="larkBrowserSessionAppId"
                label={intl.formatMessage({ id: 'pages.appConfig.security.appId' })}
                rules={[{ required: Boolean(larkEnabled) }, { max: 255 }]}
              >
                <Input autoComplete="off" />
              </Form.Item>
            </Col>
            <Col xs={24} lg={12}>
              <SecretField
                name="larkBrowserSessionAppSecret"
                label={intl.formatMessage({ id: 'pages.appConfig.security.appSecret' })}
                configured={configured.has('larkBrowserSessionAppSecret')}
                disabled={!canWriteSecrets}
                required={
                  Boolean(larkEnabled) &&
                  canWriteSecrets &&
                  !configured.has('larkBrowserSessionAppSecret')
                }
              />
            </Col>
            <Col span={24}>
              <Form.Item
                name="larkBrowserSessionRedirectURI"
                label={intl.formatMessage({ id: 'pages.appConfig.security.redirectUri' })}
                rules={[{ required: Boolean(larkEnabled) }, urlRule, { max: 2048 }]}
              >
                <Input />
              </Form.Item>
            </Col>
          </Row>
        </Card>

        {canWrite ? (
          <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
            {intl.formatMessage({ id: 'actions.save' })}
          </Button>
        ) : null}
      </Space>
    </Form>
  );
}
