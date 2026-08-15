import { LockOutlined, ReloadOutlined } from '@ant-design/icons';
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
  retryLabel = '重试',
  title = '加载失败',
}: {
  message: ReactNode;
  onRetry: () => void;
  retryLabel?: ReactNode;
  title?: ReactNode;
}) {
  return (
    <Result
      status="error"
      title={title}
      subTitle={message}
      extra={
        <Button type="primary" icon={<ReloadOutlined />} onClick={onRetry}>
          {retryLabel}
        </Button>
      }
    />
  );
}

export function PageForbidden({ message = '你没有访问此页面的权限。' }: { message?: ReactNode }) {
  return <Result status="403" icon={<LockOutlined />} title="403" subTitle={message} />;
}
