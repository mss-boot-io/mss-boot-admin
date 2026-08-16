import BellOutlined from '@ant-design/icons/BellOutlined';
import { useQuery } from '@tanstack/react-query';
import { useIntl, useModel } from '@umijs/max';
import { Alert, Switch, Typography } from 'antd';
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
  const [savedKey, setSavedKey] = useState<NotificationSettingKey>();
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
    setSavedKey(undefined);
    setMutationError(undefined);
    try {
      await accountAPI.updateNotificationSetting(key, enabled);
      await queryClient.invalidateQueries({ queryKey: queryKeys.accountNotifications(userID) });
      setSavedKey(key);
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
      <ul className="m-0 list-none divide-y divide-[var(--mss-color-split)] p-0">
        {settingKeys.map((key) => (
          <li className="flex items-center justify-between gap-4 py-4" key={key}>
            <div>
              <Typography.Text strong>
                {intl.formatMessage({ id: `account.notifications.${key}.title` })}
              </Typography.Text>
              <div id={`notification-setting-${key}-description`}>
                <Typography.Text type="secondary">
                  {intl.formatMessage({ id: `account.notifications.${key}.description` })}
                </Typography.Text>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              {savedKey === key && pendingKey !== key ? (
                <Typography.Text aria-live="polite" role="status" type="success">
                  {intl.formatMessage({ id: 'account.notifications.saved' })}
                </Typography.Text>
              ) : null}
              <Switch
                aria-describedby={`notification-setting-${key}-description`}
                aria-label={intl.formatMessage({ id: `account.notifications.${key}.title` })}
                checked={settings.data?.[key] ?? false}
                loading={pendingKey === key}
                disabled={Boolean(pendingKey && pendingKey !== key)}
                onChange={(enabled) => void update(key, enabled)}
              />
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
