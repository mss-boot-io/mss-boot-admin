import type { ProLayoutProps } from '@ant-design/pro-components';

export interface CurrentUser {
  id: string;
  username?: string;
  name?: string;
  avatar?: string;
  roleId?: string;
  root?: boolean;
  permissions?: string[];
}

export interface InitialState {
  currentUser?: CurrentUser;
  settings: Partial<ProLayoutProps>;
  authorizedMenu: AuthorizedMenuItem[];
  fetchCurrentUser: () => Promise<CurrentUser | undefined>;
}

export interface AuthorizedMenuItem {
  id?: string;
  name?: string;
  title?: string;
  path?: string;
  icon?: string;
  type?: string;
  permission?: string;
  rootOnly?: boolean;
  children?: AuthorizedMenuItem[];
}
