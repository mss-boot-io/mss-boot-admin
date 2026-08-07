import { LarkOutlined } from '@/components/MssBoot/icon';
import { deleteUserUnbinding, getUserOauth2 } from '@/services/admin/user';
import { openOAuthAuthorization } from '@/utils/oauth';
import { GithubOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import { useRequest } from 'ahooks';
import { Button, List, message } from 'antd';
import React, { Fragment, useState } from 'react';

const BindingView: React.FC = () => {
  const intl = useIntl();
  const [bindingGithub, setBindingGithub] = useState(false);
  const [bindingLark, setBindingLark] = useState(false);
  const [pendingProvider, setPendingProvider] = useState<API.OAuthProvider>();

  const { refreshAsync } = useRequest(async () => {
    const bindings = (await getUserOauth2()) || [];
    setBindingGithub(bindings.some((item) => item.type === 'github'));
    setBindingLark(bindings.some((item) => item.type === 'lark'));
    return bindings;
  });

  const bindProvider = async (provider: API.OAuthProvider) => {
    if (pendingProvider) {
      return;
    }
    setPendingProvider(provider);
    try {
      const result = await openOAuthAuthorization(provider, 'binding');
      if (result.intent !== 'binding') {
        throw new Error('OAuth binding returned the wrong intent');
      }
      await refreshAsync();
      message.success(intl.formatMessage({ id: 'pages.settings.binding.success' }));
    } catch {
      message.error(intl.formatMessage({ id: 'pages.settings.binding.failed' }));
    } finally {
      setPendingProvider(undefined);
    }
  };

  const unbindProvider = async (provider: API.OAuthProvider) => {
    setPendingProvider(provider);
    try {
      await deleteUserUnbinding({ type: provider });
      await refreshAsync();
      message.success(intl.formatMessage({ id: 'pages.settings.unbinding.success' }));
    } finally {
      setPendingProvider(undefined);
    }
  };

  const getData = () => [
    {
      title: 'Github',
      description: bindingGithub
        ? intl.formatMessage({ id: 'pages.settings.binding.github' })
        : intl.formatMessage({ id: 'pages.settings.unbinding.github' }),
      actions: [
        <Button
          key="github-binding"
          type="link"
          loading={pendingProvider === 'github'}
          disabled={Boolean(pendingProvider && pendingProvider !== 'github')}
          onClick={() =>
            void (bindingGithub ? unbindProvider('github') : bindProvider('github'))
          }
        >
          <FormattedMessage
            id={bindingGithub ? 'pages.settings.unbinding' : 'pages.settings.binding'}
          />
        </Button>,
      ],
      avatar: <GithubOutlined className="github" />,
    },
    {
      title: 'Lark',
      description: bindingLark
        ? intl.formatMessage({ id: 'pages.settings.binding.lark' })
        : intl.formatMessage({ id: 'pages.settings.unbinding.lark' }),
      actions: [
        <Button
          key="lark-binding"
          type="link"
          loading={pendingProvider === 'lark'}
          disabled={Boolean(pendingProvider && pendingProvider !== 'lark')}
          onClick={() => void (bindingLark ? unbindProvider('lark') : bindProvider('lark'))}
        >
          <FormattedMessage
            id={bindingLark ? 'pages.settings.unbinding' : 'pages.settings.binding'}
          />
        </Button>,
      ],
      avatar: <LarkOutlined className="lark" />,
    },
  ];

  return (
    <Fragment>
      <List
        itemLayout="horizontal"
        dataSource={getData()}
        renderItem={(item) => (
          <List.Item actions={item.actions}>
            <List.Item.Meta
              avatar={item.avatar}
              title={item.title}
              description={item.description}
            />
          </List.Item>
        )}
      />
    </Fragment>
  );
};

export default BindingView;
