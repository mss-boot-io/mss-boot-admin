import GithubOutlined from '@ant-design/icons/GithubOutlined';
import MenuOutlined from '@ant-design/icons/MenuOutlined';
import { Link, useIntl } from '@umijs/max';
import { Button, Typography } from 'antd';
import type { InitialState, StartupFailure } from '../auth/types';
import { PageError } from '../design-system/PageState';
import { AvatarMenu, HeaderActions } from './HeaderActions';

export function StartupFailureView({ failure }: { failure: StartupFailure }) {
  const intl = useIntl();
  const messageID =
    failure.area === 'identity'
      ? 'startup.identityUnavailable'
      : 'startup.authorizationUnavailable';
  const status = failure.status ? ` (HTTP ${failure.status})` : '';
  return (
    <PageError
      message={`${intl.formatMessage({ id: messageID })}${status}`}
      onRetry={() => window.location.reload()}
      retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      title={intl.formatMessage({ id: 'states.loadError' })}
    />
  );
}

function applicationIdentity(initialState?: InitialState) {
  return {
    title:
      typeof initialState?.settings?.title === 'string' && initialState.settings.title.trim()
        ? initialState.settings.title.trim()
        : 'mss-boot-io',
    logo:
      typeof initialState?.settings?.logo === 'string' && initialState.settings.logo.trim()
        ? initialState.settings.logo.trim()
        : '/logo.svg',
  };
}

export function ApplicationBrand({
  collapsed = false,
  initialState,
}: {
  collapsed?: boolean;
  initialState?: InitialState;
}) {
  const intl = useIntl();
  const { logo, title } = applicationIdentity(initialState);
  return (
    <Link
      aria-label={intl.formatMessage({ id: 'navigation.home' }, { name: title })}
      className="flex h-full min-w-0 items-center gap-2"
      to="/workplace"
    >
      <img alt="" className="h-7 w-7 shrink-0 object-contain" src={logo} />
      {collapsed ? null : <span className="truncate font-semibold">{title}</span>}
    </Link>
  );
}

export function AccessibleMobileHeader({
  collapsed,
  initialState,
  onCollapse,
}: {
  collapsed?: boolean;
  initialState?: InitialState;
  onCollapse?: (collapsed: boolean) => void;
}) {
  const intl = useIntl();
  const { logo, title } = applicationIdentity(initialState);
  const navigationLabel = intl.formatMessage({
    id: collapsed ? 'navigation.sidebar.open' : 'navigation.sidebar.close',
  });

  return (
    <div className="flex h-full min-w-0 flex-1 items-center gap-1 px-2">
      <Button
        aria-expanded={!collapsed}
        aria-label={navigationLabel}
        htmlType="button"
        icon={<MenuOutlined />}
        onClick={() => onCollapse?.(!collapsed)}
        type="text"
      />
      <Link
        aria-label={intl.formatMessage({ id: 'navigation.home' }, { name: title })}
        className="flex min-w-0 items-center gap-2"
        to="/workplace"
      >
        <img alt="" className="h-7 w-7 shrink-0 object-contain" src={logo} />
        <span className="hidden max-w-36 truncate font-semibold sm:inline">{title}</span>
      </Link>
      <div className="ml-auto flex shrink-0 items-center">
        <HeaderActions compact initialState={initialState} />
        <AvatarMenu compact initialState={initialState} />
      </div>
    </div>
  );
}

export function ApplicationFooter({ initialState }: { initialState?: InitialState }) {
  const base = initialState?.applicationProfile?.base;
  const copyright =
    typeof base?.websiteCopyRight === 'string' && base.websiteCopyRight.trim()
      ? base.websiteCopyRight.trim()
      : 'mss-boot-io';
  const recordNumber =
    typeof base?.websiteRecordNumber === 'string' ? base.websiteRecordNumber.trim() : '';

  return (
    <footer className="flex flex-col items-center gap-2 py-4 text-center text-sm text-neutral-500">
      <div>
        © {new Date().getFullYear()} {copyright}
      </div>
      <div className="flex flex-wrap items-center justify-center gap-x-4 gap-y-1">
        {recordNumber ? (
          <Typography.Link href="https://beian.miit.gov.cn" target="_blank" rel="noreferrer">
            {recordNumber}
          </Typography.Link>
        ) : null}
        <Typography.Link
          href="https://github.com/mss-boot-io/mss-boot"
          target="_blank"
          rel="noreferrer"
        >
          <GithubOutlined /> mss-boot
        </Typography.Link>
        <Typography.Link
          href="https://github.com/mss-boot-io/mss-boot-admin"
          target="_blank"
          rel="noreferrer"
        >
          mss-boot-admin
        </Typography.Link>
      </div>
    </footer>
  );
}
