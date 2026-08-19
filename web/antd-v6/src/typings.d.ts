declare module '*.css';

declare const __APP_VERSION__: string;
declare const __ANTD_VERSION__: string;

declare namespace API {
  type CurrentUser = import('@mss-admin-core/shared/auth/types').CurrentUser;
  type InitialState = import('@mss-admin-core/shared/auth/types').InitialState;
}
