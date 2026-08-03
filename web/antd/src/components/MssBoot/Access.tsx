import { useModel } from '@umijs/max';
import { PropsWithChildren } from 'react';

export interface AccessProps {
  accessible?: boolean;
  fallback?: React.ReactNode;
  key?: string;
}

export const Access: React.FC<PropsWithChildren<AccessProps>> = (props) => {
  const { initialState } = useModel('@@initialState');
  let accessible: boolean =
    (props.accessible ?? false) || (initialState?.currentUser?.role?.root ?? false);

  if (!accessible && props.key) {
    accessible =
      initialState?.currentUser?.role?.root ??
      !!initialState?.currentUser?.permissions?.hasOwnProperty(props.key);
  }

  return <>{accessible ? props.children : props.fallback}</>;
};
