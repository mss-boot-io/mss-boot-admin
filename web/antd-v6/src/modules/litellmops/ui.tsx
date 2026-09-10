import { getRequestErrorMessage, getRequestStatus } from '@mss-admin-core/shared/api/errors';
import {
  PageEmpty,
  PageError,
  PageForbidden,
  PageLoading,
} from '@mss-admin-core/shared/design-system/PageState';
import { useIntl } from '@umijs/max';
import { Alert, Tag } from 'antd';
import type { ReactNode } from 'react';
import type { SalesOrderStatus } from './contract';

export function OpsQueryState({
  children,
  data,
  empty,
  error,
  isError,
  isPending,
  onRetry,
  emptyDescription,
}: {
  children: ReactNode;
  data: unknown;
  empty?: boolean;
  error: unknown;
  isError: boolean;
  isPending: boolean;
  onRetry: () => void;
  emptyDescription?: ReactNode;
}) {
  const intl = useIntl();
  if (isPending && !data) return <PageLoading />;
  if (isError && !data && getRequestStatus(error) === 403) return <PageForbidden />;
  if (isError && !data) {
    return <PageError message={getRequestErrorMessage(error)} onRetry={onRetry} />;
  }
  if (empty) {
    return (
      <PageEmpty description={emptyDescription ?? intl.formatMessage({ id: 'states.empty' })} />
    );
  }
  return (
    <>
      {isError ? <Alert showIcon type="warning" message={getRequestErrorMessage(error)} /> : null}
      {children}
    </>
  );
}

const statusColors: Partial<Record<SalesOrderStatus, string>> = {
  received: 'default',
  verified_paid: 'processing',
  mapped: 'cyan',
  approved: 'blue',
  executing: 'processing',
  applied_unverified: 'gold',
  completed: 'success',
  retryable_failed: 'warning',
  terminal_failed: 'error',
  reconcile_required: 'error',
  refund_review: 'magenta',
  reversed: 'purple',
};

export function OrderStatusTag({ status }: { status: SalesOrderStatus }) {
  const intl = useIntl();
  return (
    <Tag color={statusColors[status]}>
      {intl.formatMessage({ id: `litellmops.sales.status.${status}` })}
    </Tag>
  );
}
