import ApiOutlined from '@ant-design/icons/ApiOutlined';
import GithubOutlined from '@ant-design/icons/GithubOutlined';
import KeyOutlined from '@ant-design/icons/KeyOutlined';
import SafetyCertificateOutlined from '@ant-design/icons/SafetyCertificateOutlined';
import { useQuery } from '@tanstack/react-query';
import { useIntl, useModel, useSearchParams } from '@umijs/max';
import { Alert, App, Button, Card, Descriptions, Form, Input, Space, Tag, Typography } from 'antd';
import { useState } from 'react';
import { getRequestErrorMessage } from '@/shared/api/client';
import type { InitialState } from '@/shared/auth/types';
import { PageError, PageLoading } from '@/shared/design-system/PageState';
import { queryClient, queryKeys } from '@/shared/query/client';
import { isOAuthProviderEnabled } from '@/shared/theme/contract';
import { clearUserThemeRuntime } from '@/shared/theme/runtime';
import { clearThemeIdentitySession } from '@/shared/theme/snapshot';
import { accountAPI } from './api';
import type { OAuthProvider } from './contracts';

interface PasswordProofValues {
  password: string;
}

interface PasswordChangeValues {
  newPassword: string;
  confirmPassword: string;
}

function formatExpiry(value: string | undefined, locale: string): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  }).format(date);
}

export default function SecurityPanel() {
  const intl = useIntl();
  const { message } = App.useApp();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const [searchParams, setSearchParams] = useSearchParams();
  const userID = initialState?.currentUser?.id ?? '';
  const [proofForm] = Form.useForm<PasswordProofValues>();
  const [passwordForm] = Form.useForm<PasswordChangeValues>();
  const [proving, setProving] = useState(false);
  const [changing, setChanging] = useState(false);
  const [oauthProvider, setOAuthProvider] = useState<OAuthProvider>();
  const [operationError, setOperationError] = useState<string>();

  const security = useQuery({
    queryKey: queryKeys.accountSecurity(userID),
    queryFn: accountAPI.loadSecurityStatus,
    enabled: Boolean(userID),
    staleTime: 0,
  });
  const bindings = useQuery({
    queryKey: queryKeys.accountOAuth(userID),
    queryFn: accountAPI.listOAuthBindings,
    enabled: Boolean(userID),
    staleTime: 0,
  });

  if (security.isPending && !security.data) return <PageLoading rows={6} />;
  if (security.isError || !security.data) {
    return (
      <PageError
        message={intl.formatMessage({ id: 'account.security.loadFailed' })}
        onRetry={() => void security.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }

  const reauthenticateWithPassword = async ({ password }: PasswordProofValues) => {
    if (proving) return;
    setProving(true);
    setOperationError(undefined);
    try {
      // The current password remains only in this form and this awaited call;
      // it is never stored in React Query mutation state.
      await accountAPI.reauthenticateWithPassword(password);
      proofForm.resetFields();
      await queryClient.invalidateQueries({ queryKey: queryKeys.accountSecurity(userID) });
      await security.refetch();
      void message.success(intl.formatMessage({ id: 'account.security.proofSucceeded' }));
    } catch (error) {
      setOperationError(getRequestErrorMessage(error));
    } finally {
      proofForm.resetFields();
      setProving(false);
    }
  };

  const startOAuthProof = async (provider: OAuthProvider) => {
    if (oauthProvider) return;
    setOAuthProvider(provider);
    setOperationError(undefined);
    try {
      const attempt = await accountAPI.startOAuthAuthorization(provider, 'reauthentication');
      window.location.assign(attempt.authorizeURL);
    } catch (error) {
      setOperationError(getRequestErrorMessage(error));
      setOAuthProvider(undefined);
    }
  };

  const changePassword = async ({ newPassword }: PasswordChangeValues) => {
    if (changing) return;
    setChanging(true);
    setOperationError(undefined);
    try {
      await accountAPI.changePassword(newPassword);
      passwordForm.resetFields();
      queryClient.clear();
      clearThemeIdentitySession();
      clearUserThemeRuntime();
      window.location.replace('/user/login?passwordChanged=success');
    } catch (error) {
      setOperationError(getRequestErrorMessage(error));
      passwordForm.resetFields();
      setChanging(false);
    }
  };

  const proofSucceeded = searchParams.get('reauthentication') === 'success';
  const enabledBindings = (bindings.data ?? []).filter((binding) =>
    isOAuthProviderEnabled(initialState?.applicationProfile, binding.provider),
  );

  return (
    <Space orientation="vertical" size="large" className="w-full">
      {proofSucceeded ? (
        <Alert
          closable
          showIcon
          type="success"
          title={intl.formatMessage({ id: 'account.security.oauthProofSucceeded' })}
          onClose={() => {
            const next = new URLSearchParams(searchParams);
            next.delete('reauthentication');
            setSearchParams(next);
          }}
        />
      ) : null}
      {operationError ? (
        <Alert
          closable
          showIcon
          type="error"
          title={intl.formatMessage({ id: 'account.security.operationFailed' })}
          description={operationError}
          onClose={() => setOperationError(undefined)}
        />
      ) : null}

      <Alert
        showIcon
        icon={<SafetyCertificateOutlined />}
        type="info"
        title={intl.formatMessage({ id: 'account.security.passwordOwnershipTitle' })}
        description={intl.formatMessage({ id: 'account.security.passwordOwnershipDescription' })}
      />

      <Card title={intl.formatMessage({ id: 'account.security.contactTitle' })}>
        <Alert
          className="mb-5"
          showIcon
          type="info"
          title={intl.formatMessage({ id: 'account.security.contactReadOnlyTitle' })}
          description={intl.formatMessage({ id: 'account.security.contactReadOnlyDescription' })}
        />
        <Descriptions
          column={{ xs: 1, md: 2 }}
          items={[
            {
              key: 'email',
              label: intl.formatMessage({ id: 'account.security.email' }),
              children: initialState?.currentUser?.email || (
                <Tag>{intl.formatMessage({ id: 'account.security.notSet' })}</Tag>
              ),
            },
            {
              key: 'phone',
              label: intl.formatMessage({ id: 'account.security.phone' }),
              children: initialState?.currentUser?.phone || (
                <Tag>{intl.formatMessage({ id: 'account.security.notSet' })}</Tag>
              ),
            },
          ]}
        />
      </Card>

      <Card
        title={intl.formatMessage({ id: 'account.security.identityProofTitle' })}
        extra={
          security.data.recentAuthentication ? (
            <Tag color="success">{intl.formatMessage({ id: 'account.security.proofActive' })}</Tag>
          ) : (
            <Tag>{intl.formatMessage({ id: 'account.security.proofRequired' })}</Tag>
          )
        }
      >
        {security.data.recentAuthentication ? (
          <Typography.Paragraph type="secondary">
            {intl.formatMessage(
              { id: 'account.security.proofExpires' },
              {
                value: formatExpiry(security.data.recentAuthenticationExpiresAt, intl.locale),
              },
            )}
          </Typography.Paragraph>
        ) : (
          <Space orientation="vertical" size="large" className="w-full">
            {security.data.reauthenticationLockedUntil ? (
              <Alert
                showIcon
                type="warning"
                title={intl.formatMessage({ id: 'account.security.proofLocked' })}
                description={formatExpiry(security.data.reauthenticationLockedUntil, intl.locale)}
              />
            ) : null}
            {security.data.hasLocalPassword ? (
              <Form<PasswordProofValues>
                form={proofForm}
                layout="vertical"
                onFinish={(values) => void reauthenticateWithPassword(values)}
              >
                <Form.Item
                  name="password"
                  label={intl.formatMessage({ id: 'account.security.currentPassword' })}
                  rules={[{ required: true }]}
                >
                  <Input.Password autoComplete="current-password" maxLength={128} />
                </Form.Item>
                <Button
                  type="primary"
                  htmlType="submit"
                  icon={<SafetyCertificateOutlined />}
                  loading={proving}
                  disabled={Boolean(security.data.reauthenticationLockedUntil)}
                >
                  {intl.formatMessage({ id: 'account.security.verifyPassword' })}
                </Button>
              </Form>
            ) : null}
            {enabledBindings.length ? (
              <div>
                <Typography.Paragraph type="secondary">
                  {intl.formatMessage({ id: 'account.security.oauthProofDescription' })}
                </Typography.Paragraph>
                <Space wrap>
                  {enabledBindings.map((binding) => (
                    <Button
                      key={binding.provider}
                      icon={binding.provider === 'github' ? <GithubOutlined /> : <ApiOutlined />}
                      loading={oauthProvider === binding.provider}
                      disabled={Boolean(oauthProvider && oauthProvider !== binding.provider)}
                      onClick={() => void startOAuthProof(binding.provider)}
                    >
                      {intl.formatMessage(
                        { id: 'account.security.verifyWithProvider' },
                        { provider: binding.provider === 'github' ? 'GitHub' : 'Lark' },
                      )}
                    </Button>
                  ))}
                </Space>
              </div>
            ) : null}
            {!security.data.hasLocalPassword && !enabledBindings.length ? (
              <Alert
                showIcon
                type="warning"
                title={intl.formatMessage({ id: 'account.security.noProofMethod' })}
              />
            ) : null}
          </Space>
        )}
      </Card>

      <Card title={intl.formatMessage({ id: 'account.security.changePasswordTitle' })}>
        <Alert
          className="mb-5"
          showIcon
          type="warning"
          title={intl.formatMessage({ id: 'account.security.signOutWarningTitle' })}
          description={intl.formatMessage({ id: 'account.security.signOutWarningDescription' })}
        />
        <Form<PasswordChangeValues>
          form={passwordForm}
          layout="vertical"
          disabled={!security.data.recentAuthentication}
          onFinish={(values) => void changePassword(values)}
        >
          <Form.Item
            name="newPassword"
            label={intl.formatMessage({ id: 'account.security.newPassword' })}
            extra={intl.formatMessage({ id: 'account.security.passwordPolicy' })}
            rules={[
              { required: true },
              { min: 8, max: 128 },
              {
                pattern: /^(?=.*\p{L})(?=.*\p{N}).+$/u,
                message: intl.formatMessage({ id: 'account.security.passwordPolicy' }),
              },
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            label={intl.formatMessage({ id: 'account.security.confirmPassword' })}
            dependencies={['newPassword']}
            rules={[
              { required: true },
              ({ getFieldValue }) => ({
                validator: (_, value) =>
                  !value || getFieldValue('newPassword') === value
                    ? Promise.resolve()
                    : Promise.reject(
                        new Error(intl.formatMessage({ id: 'account.security.passwordMismatch' })),
                      ),
              }),
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Button danger type="primary" htmlType="submit" icon={<KeyOutlined />} loading={changing}>
            {intl.formatMessage({ id: 'account.security.changePassword' })}
          </Button>
        </Form>
      </Card>
    </Space>
  );
}
