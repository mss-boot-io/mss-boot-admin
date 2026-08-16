import LockOutlined from '@ant-design/icons/LockOutlined';
import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import { useIntl } from '@umijs/max';
import { Button, Empty, Result, Skeleton } from 'antd';
import type { ReactNode } from 'react';

export function PageLoading({ rows = 5 }: { rows?: number }) {
  return (
    <div aria-busy="true" role="status">
      <Skeleton active paragraph={{ rows }} title />
    </div>
  );
}

export function PageEmpty({ description }: { description: ReactNode }) {
  return <Empty description={description} image={Empty.PRESENTED_IMAGE_SIMPLE} />;
}

export function PageError({
  message,
  onRetry,
  retryLabel,
  title,
}: {
  message: ReactNode;
  onRetry: () => void;
  retryLabel?: ReactNode;
  title?: ReactNode;
}) {
  const intl = useIntl();
  return (
    <Result
      status="error"
      title={title ?? intl.formatMessage({ id: 'states.loadError' })}
      subTitle={message}
      extra={
        <Button type="primary" icon={<ReloadOutlined />} onClick={onRetry}>
          {retryLabel ?? intl.formatMessage({ id: 'actions.retry' })}
        </Button>
      }
    />
  );
}

export function PageForbidden({ message }: { message?: ReactNode }) {
  const intl = useIntl();
  return (
    <Result
      status="403"
      icon={<LockOutlined />}
      title="403"
      subTitle={message ?? intl.formatMessage({ id: 'states.forbidden' })}
    />
  );
}
