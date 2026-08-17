import type { RunTimeLayoutConfig } from '@umijs/max';
import { history, Link } from '@umijs/max';
import { isPublicPath, redirectToLogin } from '../auth/session';
import { resolveMenuIcons } from '../navigation/menuIcons';
import { AvatarMenu, desktopHeaderActions } from './HeaderActions';
import {
  AccessibleMobileHeader,
  ApplicationBrand,
  ApplicationFooter,
  StartupFailureView,
} from './LayoutChrome';

export const runtimeLayout: RunTimeLayoutConfig = ({ initialState }) => ({
  ...initialState?.settings,
  actionsRender: () => desktopHeaderActions(initialState),
  avatarProps: {
    render: () => <AvatarMenu initialState={initialState} />,
  },
  breadcrumbRender: (routers = []) => routers,
  childrenRender: (children) =>
    initialState?.startupFailure ? (
      <StartupFailureView failure={initialState.startupFailure} />
    ) : (
      children
    ),
  footerRender: () => <ApplicationFooter initialState={initialState} />,
  headerRender: (props, defaultDom) =>
    props.isMobile ? (
      <AccessibleMobileHeader
        collapsed={props.collapsed}
        initialState={initialState}
        onCollapse={props.onCollapse}
      />
    ) : (
      defaultDom
    ),
  headerTitleRender: (_logo, _title, props) => (
    <ApplicationBrand collapsed={props?.collapsed} initialState={initialState} />
  ),
  menu: {
    params: { authorizationVersion: initialState?.authorizationVersion ?? 0 },
    request: async () => initialState?.authorizedMenu ?? [],
  },
  menuDataRender: resolveMenuIcons,
  menuHeaderRender: (_logo, _title, props) => (
    <ApplicationBrand collapsed={props?.collapsed} initialState={initialState} />
  ),
  menuItemRender: (item, dom) =>
    item.path ? (
      <Link to={item.path} prefetch>
        {dom}
      </Link>
    ) : (
      dom
    ),
  onPageChange: () => {
    if (
      !initialState?.startupFailure &&
      !initialState?.currentUser &&
      !isPublicPath(history.location.pathname)
    ) {
      redirectToLogin();
    }
  },
  pageTitleRender: false,
  waterMarkProps: {
    content:
      initialState?.currentUser?.name?.trim() || initialState?.currentUser?.username?.trim() || '',
    gapX: 320,
    gapY: 240,
    font: {
      color:
        initialState?.settings?.navTheme === 'realDark'
          ? 'rgba(255, 255, 255, 0.035)'
          : 'rgba(0, 0, 0, 0.025)',
      fontSize: 12,
    },
  },
});
