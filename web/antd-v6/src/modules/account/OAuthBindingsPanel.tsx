import ApiOutlined from '@ant-design/icons/ApiOutlined';
import GithubOutlined from '@ant-design/icons/GithubOutlined';
import SafetyCertificateOutlined from '@ant-design/icons/SafetyCertificateOutlined';
import { useQuery } from '@tanstack/react-query';
import { useIntl, useModel, useSearchParams } from '@umijs/max';
import { Alert, Avatar, Button, List, Space, Tag, Typography } from 'antd';
import { useState } from 'react';
import { getRequestErrorMessage } from '@/shared/api/client';
import type { InitialState } from '@/shared/auth/types';
import { PageError, PageLoading } from '@/shared/design-system/PageState';
import { queryKeys } from '@/shared/query/client';
import { isOAuthProviderEnabled } from '@/shared/theme/contract';
import { accountAPI } from './api';
import type { OAuthProvider } from './contracts';

const providers: readonly OAuthProvider[] = ['github', 'lark'];

export default function OAuthBindingsPanel() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const [searchParams, setSearchParams] = useSearchParams();
  const userID = initialState?.currentUser?.id ?? '';
  const [pendingProvider, setPendingProvider] = useState<OAuthProvider>();
  const [bindingError, setBindingError] = useState<string>();
  const bindings = useQuery({
    queryKey: queryKeys.accountOAuth(userID),
    queryFn: accountAPI.listOAuthBindings,
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
      <List
        dataSource={visibleProviders}
        locale={{ emptyText: intl.formatMessage({ id: 'account.oauth.noneAvailable' }) }}
        renderItem={(provider) => {
          const binding = bindings.data?.find((item) => item.provider === provider);
          const enabled = isOAuthProviderEnabled(initialState?.applicationProfile, provider);
          return (
            <List.Item
              actions={[
                binding ? (
                  <Tag key="bound" color="success">
                    {intl.formatMessage({ id: 'account.oauth.statusBound' })}
                  </Tag>
                ) : enabled ? (
                  <Button
                    key="bind"
                    type="primary"
                    loading={pendingProvider === provider}
                    disabled={Boolean(pendingProvider && pendingProvider !== provider)}
                    onClick={() => void startBinding(provider)}
                  >
                    {intl.formatMessage({ id: 'account.oauth.bind' })}
                  </Button>
                ) : (
                  <Tag key="disabled">
                    {intl.formatMessage({ id: 'account.oauth.statusDisabled' })}
                  </Tag>
                ),
              ]}
            >
              <List.Item.Meta
                avatar={
                  binding?.picture ? (
                    <Avatar src={binding.picture} />
                  ) : (
                    <Avatar icon={provider === 'github' ? <GithubOutlined /> : <ApiOutlined />} />
                  )
                }
                title={provider === 'github' ? 'GitHub' : 'Lark'}
                description={
                  binding ? (
                    <Space orientation="vertical" size={0}>
                      <Typography.Text>
                        {binding.displayName ?? binding.email ?? '—'}
                      </Typography.Text>
                      {binding.email && binding.displayName ? (
                        <Typography.Text type="secondary">{binding.email}</Typography.Text>
                      ) : null}
                    </Space>
                  ) : (
                    intl.formatMessage({ id: 'account.oauth.notBound' })
                  )
                }
              />
            </List.Item>
          );
        }}
      />
    </Space>
  );
}
