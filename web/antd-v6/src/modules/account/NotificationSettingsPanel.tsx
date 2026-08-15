import { BellOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { useIntl, useModel } from '@umijs/max';
import { Alert, List, Switch } from 'antd';
import { useState } from 'react';
import { getRequestErrorMessage } from '@/shared/api/client';
import type { InitialState } from '@/shared/auth/types';
import { PageError, PageLoading } from '@/shared/design-system/PageState';
import { queryClient, queryKeys } from '@/shared/query/client';
import { accountAPI } from './api';
import type { NotificationSettingKey } from './contracts';

const settingKeys: readonly NotificationSettingKey[] = ['password', 'system', 'todo', 'email'];

export default function NotificationSettingsPanel() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const userID = initialState?.currentUser?.id ?? '';
  const [pendingKey, setPendingKey] = useState<NotificationSettingKey>();
  const [mutationError, setMutationError] = useState<string>();
  const settings = useQuery({
    queryKey: queryKeys.accountNotifications(userID),
    queryFn: accountAPI.loadNotificationSettings,
    enabled: Boolean(userID),
    staleTime: 0,
  });

  if (settings.isPending && !settings.data) return <PageLoading rows={4} />;
  if (settings.isError && !settings.data) {
    return (
      <PageError
        message={intl.formatMessage({ id: 'account.notifications.loadFailed' })}
        onRetry={() => void settings.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }

  const update = async (key: NotificationSettingKey, enabled: boolean) => {
    if (pendingKey) return;
    setPendingKey(key);
    setMutationError(undefined);
    try {
      await accountAPI.updateNotificationSetting(key, enabled);
      await queryClient.invalidateQueries({ queryKey: queryKeys.accountNotifications(userID) });
    } catch (error) {
      setMutationError(getRequestErrorMessage(error));
    } finally {
      setPendingKey(undefined);
    }
  };

  return (
    <div>
      <Alert
        className="mb-4"
        showIcon
        icon={<BellOutlined />}
        type="info"
        title={intl.formatMessage({ id: 'account.notifications.description' })}
      />
      {mutationError ? (
        <Alert
          className="mb-4"
          closable
          showIcon
          type="error"
          title={intl.formatMessage({ id: 'account.notifications.saveFailed' })}
          description={mutationError}
          onClose={() => setMutationError(undefined)}
        />
      ) : null}
      <List
        dataSource={[...settingKeys]}
        renderItem={(key) => (
          <List.Item
            actions={[
              <Switch
                key={key}
                checked={settings.data?.[key] ?? false}
                loading={pendingKey === key}
                disabled={Boolean(pendingKey && pendingKey !== key)}
                onChange={(enabled) => void update(key, enabled)}
              />,
            ]}
          >
            <List.Item.Meta
              title={intl.formatMessage({ id: `account.notifications.${key}.title` })}
              description={intl.formatMessage({ id: `account.notifications.${key}.description` })}
            />
          </List.Item>
        )}
      />
    </div>
  );
}
