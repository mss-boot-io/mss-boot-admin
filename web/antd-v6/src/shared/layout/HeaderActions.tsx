import LogoutOutlined from '@ant-design/icons/LogoutOutlined';
import QuestionCircleOutlined from '@ant-design/icons/QuestionCircleOutlined';
import SettingOutlined from '@ant-design/icons/SettingOutlined';
import UserOutlined from '@ant-design/icons/UserOutlined';
import { history, Link, useIntl } from '@umijs/max';
import { Avatar, Button, Dropdown, Typography } from 'antd';
import { clearServerSession } from '../auth/session';
import type { InitialState } from '../auth/types';
import HeaderNotice from '../navigation/HeaderNotice';
import LocaleSwitcher from '../navigation/LocaleSwitcher';
import MenuSearch from '../navigation/MenuSearch';
import { queryClient } from '../query/client';
import { clearUserThemeRuntime } from '../theme/runtime';

export function AvatarMenu({
  initialState,
  compact = false,
}: {
  initialState?: InitialState;
  compact?: boolean;
}) {
  const intl = useIntl();
  const currentUser = initialState?.currentUser;
  const label = currentUser?.name ?? currentUser?.username ?? 'MSS User';
  return (
    <Dropdown
      menu={{
        items: [
          {
            key: 'center',
            icon: <UserOutlined />,
            label: (
              <Link to="/account/center">{intl.formatMessage({ id: 'menu.account-center' })}</Link>
            ),
          },
          {
            key: 'settings',
            icon: <SettingOutlined />,
            label: (
              <Link to="/account/settings">
                {intl.formatMessage({ id: 'menu.account-settings' })}
              </Link>
            ),
          },
          {
            key: 'logout',
            icon: <LogoutOutlined />,
            label: intl.formatMessage({ id: 'menu.logout' }),
            onClick: async () => {
              const { clearThemeIdentitySession } = await import('../theme/snapshot');
              const previousSessionID = clearThemeIdentitySession({
                broadcast: false,
                expectedSessionId: initialState?.authSessionId,
              });
              queryClient.clear();
              clearUserThemeRuntime();
              try {
                await clearServerSession();
              } finally {
                if (previousSessionID) {
                  void import('../theme/sync')
                    .then(({ publishThemeIdentityCleared }) =>
                      publishThemeIdentityCleared(previousSessionID),
                    )
                    .catch(() => {});
                }
                history.replace('/user/login');
              }
            },
          },
        ],
      }}
    >
      <Button
        aria-haspopup="menu"
        aria-label={intl.formatMessage({ id: 'navigation.accountMenu' }, { name: label })}
        className="h-auto px-1 py-0"
        htmlType="button"
        icon={
          <Avatar src={currentUser?.avatar || undefined}>{label.slice(0, 1).toUpperCase()}</Avatar>
        }
        type="text"
      >
        {compact ? null : <Typography.Text>{label}</Typography.Text>}
      </Button>
    </Dropdown>
  );
}

function DocumentationLink() {
  const intl = useIntl();
  return (
    <Button
      type="text"
      href="https://docs.mss-boot-io.top"
      target="_blank"
      rel="noreferrer"
      icon={<QuestionCircleOutlined />}
      aria-label={intl.formatMessage({ id: 'navigation.documentation' })}
    />
  );
}

export function HeaderActions({
  initialState,
  compact = false,
}: {
  initialState?: InitialState;
  compact?: boolean;
}) {
  return (
    <>
      <MenuSearch items={initialState?.authorizedMenu ?? []} />
      <HeaderNotice user={initialState?.currentUser} />
      {compact ? null : <DocumentationLink />}
      <LocaleSwitcher />
    </>
  );
}

export function desktopHeaderActions(initialState?: InitialState) {
  return [
    <MenuSearch key="menu-search" items={initialState?.authorizedMenu ?? []} />,
    <HeaderNotice key="notice" user={initialState?.currentUser} />,
    <DocumentationLink key="documentation" />,
    <LocaleSwitcher key="language" />,
  ];
}
