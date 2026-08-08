import { useModel } from '@umijs/max';
import React, { PropsWithChildren } from 'react';
import { hasPermission, isRootIdentity } from '@/utils/authorization';

export interface AccessProps {
  accessible?: boolean;
  fallback?: React.ReactNode;
  permission?: string;
  rootOnly?: boolean;
}

export const Access: React.FC<PropsWithChildren<AccessProps>> = (props) => {
  const { initialState } = useModel('@@initialState');
  const currentUser = initialState?.currentUser;
  const accessible = props.rootOnly
    ? isRootIdentity(currentUser)
    : props.accessible === true || hasPermission(currentUser, props.permission);

  return <>{accessible ? props.children : props.fallback}</>;
};
