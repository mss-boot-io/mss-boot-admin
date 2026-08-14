import { LockOutlined, ReloadOutlined } from '@ant-design/icons';
import { Button, Empty, Result, Skeleton } from 'antd';
import type { ReactNode } from 'react';

export function PageLoading({ rows = 5 }: { rows?: number }) {
  return <Skeleton active paragraph={{ rows }} title />;
}

export function PageEmpty({ description }: { description: ReactNode }) {
  return <Empty description={description} image={Empty.PRESENTED_IMAGE_SIMPLE} />;
}

export function PageError({ message, onRetry }: { message: ReactNode; onRetry: () => void }) {
  return (
    <Result
      status="error"
      title="加载失败"
      subTitle={message}
      extra={
        <Button type="primary" icon={<ReloadOutlined />} onClick={onRetry}>
          重试
        </Button>
      }
    />
  );
}

export function PageForbidden() {
  return (
    <Result status="403" icon={<LockOutlined />} title="403" subTitle="你没有访问此页面的权限。" />
  );
}
