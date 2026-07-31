import { List, Switch } from 'antd';
import React, { Fragment, useState } from 'react';
import { getUserConfigsGroup, putUserConfigsGroup } from '@/services/admin/userConfig';
import { useRequest } from 'ahooks';
import { fieldIntl } from '@/util/fieldIntl';
import { useIntl } from '@umijs/max';

type Unpacked<T> = T extends (infer U)[] ? U : T;

const NotificationView: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const [refresh, setRefresh] = useState(0);

  const { data, loading } = useRequest(
    async () => {
      return await getUserConfigsGroup({ group: 'notification' });
    },
    {
      refreshDeps: [refresh],
    },
  );

  const getData = () => {
    return [
      {
        title: fieldIntl(intl, 'accountNotice'),
        description: fieldIntl(intl, 'accountNotice.description'),
        actions: [
          <Switch
            key="password"
            checkedChildren={intl.formatMessage({ id: 'pages.settings.account.notification.open' })}
            unCheckedChildren={intl.formatMessage({
              id: 'pages.settings.account.notification.close',
            })}
            onChange={async (e) => {
              await putUserConfigsGroup({ group: 'notification' }, { data: { password: e } });
              setRefresh(refresh + 1);
            }}
            value={data?.password === 'true'}
            loading={loading}
          />,
        ],
      },
      {
        title: fieldIntl(intl, 'systemNotice'),
        description: fieldIntl(intl, 'systemNotice.description'),
        // description: '系统消息将以站内信的形式通知',
        actions: [
          <Switch
            key="system"
            checkedChildren={intl.formatMessage({ id: 'pages.settings.account.notification.open' })}
            unCheckedChildren={intl.formatMessage({
              id: 'pages.settings.account.notification.close',
            })}
            onChange={async (e) => {
              await putUserConfigsGroup({ group: 'notification' }, { data: { system: e } });
              setRefresh(refresh + 1);
            }}
            value={data?.system === 'true'}
            loading={loading}
          />,
        ],
      },
      {
        title: fieldIntl(intl, 'todoNotice'),
        description: fieldIntl(intl, 'todoNotice.description'),
        actions: [
          <Switch
            key="todo"
            checkedChildren={intl.formatMessage({ id: 'pages.settings.account.notification.open' })}
            unCheckedChildren={intl.formatMessage({
              id: 'pages.settings.account.notification.close',
            })}
            onChange={async (e) => {
              await putUserConfigsGroup({ group: 'notification' }, { data: { todo: e } });
              setRefresh(refresh + 1);
            }}
            value={data?.todo === 'true'}
            loading={loading}
          />,
        ],
      },
      {
        title: fieldIntl(intl, 'emailNotice'),
        description: fieldIntl(intl, 'emailNotice.description'),
        actions: [
          <Switch
            key="email"
            checkedChildren={intl.formatMessage({ id: 'pages.settings.account.notification.open' })}
            unCheckedChildren={intl.formatMessage({
              id: 'pages.settings.account.notification.close',
            })}
            onChange={async (e) => {
              await putUserConfigsGroup({ group: 'notification' }, { data: { email: e } });
              setRefresh(refresh + 1);
            }}
            value={data?.email === 'true'}
            loading={loading}
          />,
        ],
      },
    ];
  };

  const dataSource = getData();
  return (
    <Fragment>
      <List<Unpacked<typeof dataSource>>
        itemLayout="horizontal"
        dataSource={dataSource}
        renderItem={(item) => (
          <List.Item key={`${item.title}-${refresh}`} actions={item.actions}>
            <List.Item.Meta title={item.title} description={item.description} />
          </List.Item>
        )}
      />
    </Fragment>
  );
};

export default NotificationView;
