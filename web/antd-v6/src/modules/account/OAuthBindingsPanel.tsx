import ApiOutlined from '@ant-design/icons/ApiOutlined';
import DeleteOutlined from '@ant-design/icons/DeleteOutlined';
import GithubOutlined from '@ant-design/icons/GithubOutlined';
import SafetyCertificateOutlined from '@ant-design/icons/SafetyCertificateOutlined';
import { useQuery } from '@tanstack/react-query';
import { useIntl, useModel, useSearchParams } from '@umijs/max';
import { Alert, App, Avatar, Button, Empty, Popconfirm, Space, Tag, Typography } from 'antd';
import { useState } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/client';
import type { InitialState } from '@/shared/auth/types';
import { PageError, PageLoading } from '@/shared/design-system/PageState';
import { queryClient, queryKeys } from '@/shared/query/client';
import { isOAuthProviderEnabled } from '@/shared/theme/contract';
import { accountAPI } from './api';
import type { OAuthProvider } from './contracts';

const providers: readonly OAuthProvider[] = ['github', 'lark'];

export default function OAuthBindingsPanel() {
  const intl = useIntl();
  const { message } = App.useApp();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const [searchParams, setSearchParams] = useSearchParams();
  const userID = initialState?.currentUser?.id ?? '';
  const [pendingProvider, setPendingProvider] = useState<OAuthProvider>();
  const [unlinkingProvider, setUnlinkingProvider] = useState<OAuthProvider>();
  const [bindingError, setBindingError] = useState<string>();
  const bindings = useQuery({
    queryKey: queryKeys.accountOAuth(userID),
    queryFn: accountAPI.listOAuthBindings,
    enabled: Boolean(userID),
    staleTime: 0,
  });
  const security = useQuery({
    queryKey: queryKeys.accountSecurity(userID),
    queryFn: accountAPI.loadSecurityStatus,
    enabled: Boolean(userID),
    staleTime: 0,
  });
  const visibleProviders = providers.filter(
    (provider) =>
      isOAuthProviderEnabled(initialState?.applicationProfile, provider) ||
      bindings.data?.some((binding) => binding.provider === provider),
  );

  if (bindings.isPending && !bindings.data) return <PageLoading rows={3} />;
  if (bindings.isError && !bindings.data) {
    return (
      <PageError
        message={intl.formatMessage({ id: 'account.oauth.loadFailed' })}
        onRetry={() => void bindings.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }

  const startBinding = async (provider: OAuthProvider) => {
    if (pendingProvider) return;
    setPendingProvider(provider);
    setBindingError(undefined);
    try {
      const attempt = await accountAPI.startOAuthAuthorization(provider, 'binding');
      window.location.assign(attempt.authorizeURL);
    } catch (error) {
      setBindingError(getRequestErrorMessage(error));
      setPendingProvider(undefined);
    }
  };

  const bindingSucceeded = searchParams.get('binding') === 'success';

  const openSecurityProof = () => {
    const next = new URLSearchParams(searchParams);
    next.delete('binding');
    next.set('tab', 'security');
    setSearchParams(next);
  };

  const disconnect = async (provider: OAuthProvider) => {
    if (unlinkingProvider) return;
    setUnlinkingProvider(provider);
    setBindingError(undefined);
    try {
      await accountAPI.disconnectOAuth(provider);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.accountOAuth(userID) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.accountSecurity(userID) }),
      ]);
      void message.success(intl.formatMessage({ id: 'account.oauth.unlinked' }));
    } catch (error) {
      setBindingError(getRequestErrorMessage(error));
      if (getRequestStatus(error) === 428) openSecurityProof();
    } finally {
      setUnlinkingProvider(undefined);
    }
  };

  return (
    <Space orientation="vertical" size="large" className="w-full">
      {bindingSucceeded ? (
        <Alert
          closable
          showIcon
          type="success"
          title={intl.formatMessage({ id: 'account.oauth.bound' })}
          onClose={() => {
            const next = new URLSearchParams(searchParams);
            next.delete('binding');
            setSearchParams(next);
          }}
        />
      ) : null}
      {bindingError ? (
        <Alert
          closable
          showIcon
          type="error"
          title={intl.formatMessage({ id: 'account.oauth.bindFailed' })}
          description={bindingError}
          onClose={() => setBindingError(undefined)}
        />
      ) : null}
      <Alert
        showIcon
        icon={<SafetyCertificateOutlined />}
        type="warning"
        title={intl.formatMessage({ id: 'account.oauth.unlinkGuardTitle' })}
        description={intl.formatMessage({ id: 'account.oauth.unlinkGuardDescription' })}
      />
      {visibleProviders.length === 0 ? (
        <Empty description={intl.formatMessage({ id: 'account.oauth.noneAvailable' })} />
      ) : (
        <ul className="m-0 list-none divide-y divide-[var(--mss-color-split)] p-0">
          {visibleProviders.map((provider) => {
            const binding = bindings.data?.find((item) => item.provider === provider);
            const enabled = isOAuthProviderEnabled(initialState?.applicationProfile, provider);
            return (
              <li className="flex flex-wrap items-center justify-between gap-4 py-4" key={provider}>
                <div className="flex min-w-0 items-center gap-3">
                  {binding?.picture ? (
                    <Avatar src={binding.picture} />
                  ) : (
                    <Avatar icon={provider === 'github' ? <GithubOutlined /> : <ApiOutlined />} />
                  )}
                  <div className="min-w-0">
                    <Typography.Text strong>
                      {provider === 'github' ? 'GitHub' : 'Lark'}
                    </Typography.Text>
                    <div>
                      {binding ? (
                        <Space orientation="vertical" size={0}>
                          <Typography.Text>
                            {binding.displayName ?? binding.email ?? '—'}
                          </Typography.Text>
                          {binding.email && binding.displayName ? (
                            <Typography.Text type="secondary">{binding.email}</Typography.Text>
                          ) : null}
                        </Space>
                      ) : (
                        <Typography.Text type="secondary">
                          {intl.formatMessage({ id: 'account.oauth.notBound' })}
                        </Typography.Text>
                      )}
                    </div>
                  </div>
                </div>
                <div>
                  {binding ? (
                    <Space wrap>
                      <Tag color="success">
                        {intl.formatMessage({ id: 'account.oauth.statusBound' })}
                      </Tag>
                      {security.data?.recentAuthentication ? (
                        <Popconfirm
                          title={intl.formatMessage({ id: 'account.oauth.unlinkConfirmTitle' })}
                          description={intl.formatMessage({
                            id: 'account.oauth.unlinkConfirmDescription',
                          })}
                          okButtonProps={{ danger: true }}
                          onConfirm={() => disconnect(provider)}
                        >
                          <Button
                            danger
                            icon={<DeleteOutlined />}
                            loading={unlinkingProvider === provider}
                            disabled={Boolean(unlinkingProvider && unlinkingProvider !== provider)}
                          >
                            {intl.formatMessage({ id: 'account.oauth.unlink' })}
                          </Button>
                        </Popconfirm>
                      ) : (
                        <Button
                          icon={<SafetyCertificateOutlined />}
                          loading={security.isPending}
                          onClick={openSecurityProof}
                        >
                          {intl.formatMessage({ id: 'account.oauth.verifyBeforeUnlink' })}
                        </Button>
                      )}
                    </Space>
                  ) : enabled ? (
                    <Button
                      type="primary"
                      loading={pendingProvider === provider}
                      disabled={Boolean(pendingProvider && pendingProvider !== provider)}
                      onClick={() => void startBinding(provider)}
                    >
                      {intl.formatMessage({ id: 'account.oauth.bind' })}
                    </Button>
                  ) : (
                    <Tag>{intl.formatMessage({ id: 'account.oauth.statusDisabled' })}</Tag>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </Space>
  );
}
