import type { ProLayoutProps } from '@ant-design/pro-components';
import type { ApplicationProfile } from '../theme/contract';

export interface CurrentUserRole {
  id?: string;
  name?: string;
  root: boolean;
  default?: boolean;
  status?: string;
}

export interface CurrentUserOrganization {
  id?: string;
  name?: string;
}

export interface CurrentUser {
  id: string;
  username?: string;
  name?: string;
  avatar?: string;
  roleID?: string;
  role?: CurrentUserRole;
  email?: string;
  phone?: string;
  signature?: string;
  title?: string;
  group?: string;
  country?: string;
  province?: string;
  city?: string;
  address?: string;
  profile?: string;
  tags?: readonly string[];
  departmentID?: string;
  department?: CurrentUserOrganization;
  postID?: string;
  post?: CurrentUserOrganization;
  permissions: Readonly<Record<string, boolean>>;
}

export type StartupFailureArea = 'identity' | 'authorization';

export interface StartupFailure {
  area: StartupFailureArea;
  status?: number;
}

export interface InitialState {
  currentUser?: CurrentUser;
  /** Sanitized public application profile; secrets are removed server-side. */
  applicationProfile?: ApplicationProfile;
  /** Changes only when the executable authorization projection must be reloaded. */
  authorizationVersion?: number;
  settings: Partial<ProLayoutProps>;
  authorizedMenu: AuthorizedMenuItem[];
  fetchCurrentUser: () => Promise<CurrentUser | undefined>;
  /** Random browser-only binding for non-sensitive personal theme derivatives. */
  authSessionId?: string;
  startupFailure?: StartupFailure;
  themeDegradedScopes?: Array<'application' | 'user'>;
}

export interface AuthorizedMenuItem {
  id?: string;
  key?: string;
  name?: string;
  title?: string;
  path?: string;
  sourcePath?: string;
  icon?: string;
  type?: string;
  permission?: string;
  rootOnly?: boolean;
  hideChildrenInMenu?: boolean;
  hideInMenu?: boolean;
  hideInBreadcrumb?: boolean;
  flatMenu?: boolean;
  children?: AuthorizedMenuItem[];
}
