import { useModel } from '@umijs/max';
import React, { PropsWithChildren } from 'react';

export interface AccessProps {
  accessible?: boolean;
  fallback?: React.ReactNode;
  permission?: string;
}

export const Access: React.FC<PropsWithChildren<AccessProps>> = (props) => {
  const { initialState } = useModel('@@initialState');
  const currentUser = initialState?.currentUser;
  const permissions = currentUser?.permissions;
  const hasPermission =
    !!props.permission &&
    !!permissions &&
    Object.prototype.hasOwnProperty.call(permissions, props.permission);

  const accessible = props.accessible === true || currentUser?.role?.root === true || hasPermission;

  return <>{accessible ? props.children : props.fallback}</>;
};
