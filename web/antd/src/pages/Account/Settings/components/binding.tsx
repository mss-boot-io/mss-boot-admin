import { deleteUserUnbinding, getUserOauth2, postUserBinding } from '@/services/admin/user';
import { GithubOutlined } from '@ant-design/icons';
import { FormattedMessage } from '@umijs/max';
import { useRequest } from 'ahooks';
import { List, message } from 'antd';
import React, { Fragment, useEffect, useState } from 'react';
import { LarkOutlined } from '@/components/MssBoot/icon';
import { useIntl } from '@umijs/max';
import { openOAuthAuthorization } from '@/utils/oauth';

const BindingView: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const [bindingGithub, setBindingGithub] = useState(false);
  const [bindingLark, setBindingLark] = useState(false);

  const {} = useRequest(
    async () => {
      const res = await getUserOauth2();
      if (res.length > 0) {
        res.forEach((item: any) => {
          if (item.type === 'github') {
            setBindingGithub(true);
            return;
          }
          if (item.type === 'lark') {
            setBindingLark(true);
            return;
          }
        });
      }
    },
    {
      refreshDeps: [],
    },
  );

  const getData = () => [
    {
      title: 'Github',
      description: bindingGithub
        ? intl.formatMessage({ id: 'pages.settings.binding.github' })
        : intl.formatMessage({ id: 'pages.settings.unbinding.github' }),
      actions: [
        bindingGithub ? (
          <a
            key="Bind"
            onClick={() => {
              deleteUserUnbinding({ type: 'github' }).then(() => {
                message.success(intl.formatMessage({ id: 'pages.settings.unbinding.success' }));
                setBindingGithub(false);
              });
            }}
            // href={githubURL}
            rel="noopener noreferrer"
          >
            <FormattedMessage id="pages.settings.unbinding" />
          </a>
        ) : (
          <a
            key="Bind"
            onClick={async () => {
              try {
                await openOAuthAuthorization('github', 'binding');
              } catch {
                message.error(intl.formatMessage({ id: 'pages.settings.binding.failed' }));
              }
            }}
            target="_blank"
            rel="noopener noreferrer"
          >
            <FormattedMessage id="pages.settings.binding" />
          </a>
        ),
      ],
      avatar: <GithubOutlined className="github" />,
    },
    {
      title: 'Lark',
      description: bindingLark
        ? intl.formatMessage({ id: 'pages.settings.binding.lark' })
        : intl.formatMessage({ id: 'pages.settings.unbinding.lark' }),
      actions: [
        bindingLark ? (
          <a
            key="Bind"
            onClick={() => {
              deleteUserUnbinding({ type: 'lark' }).then(() => {
                message.success(intl.formatMessage({ id: 'pages.settings.unbinding.success' }));
                setBindingLark(false);
              });
            }}
            rel="noopener noreferrer"
          >
            <FormattedMessage id="pages.settings.unbinding" />
          </a>
        ) : (
          <a
            key="Bind"
            onClick={async () => {
              try {
                await openOAuthAuthorization('lark', 'binding');
              } catch {
                message.error(intl.formatMessage({ id: 'pages.settings.binding.failed' }));
              }
            }}
            target="_blank"
            rel="noopener noreferrer"
          >
            <FormattedMessage id="pages.settings.binding" />
          </a>
        ),
      ],
      avatar: <LarkOutlined className="lark" />,
    },
  ];

  useEffect(() => {
    const handleVisibilityChange = () => {
      if (!document.hidden) {
        const bindingType = localStorage.getItem('bindingType');
        let token: string | null = null;
        let setHandler = setBindingGithub;
        switch (bindingType) {
          case 'github':
            token = localStorage.getItem('github.token');
            setHandler = setBindingGithub;
            break;
          case 'lark':
            token = localStorage.getItem('lark.token');
            setHandler = setBindingLark;
            break;
        }
        if (!bindingType) {
          return;
        }
        postUserBinding({ type: bindingType as API.LoginProvider, password: token as string }).then(
          () => {
            setHandler(true);
          },
        );
        localStorage.removeItem('bindingType');
      }
    };

    // 添加事件监听器
    document.addEventListener('visibilitychange', handleVisibilityChange);

    // 清理事件监听器
    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [setBindingGithub]);

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
