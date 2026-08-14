import { history } from '@umijs/max';
import { Button, Result } from 'antd';

export default function NotFoundPage() {
  return (
    <Result
      status="404"
      title="404"
      subTitle="页面不存在，或该能力尚未注册到 Ant Design 6 应用。"
      extra={
        <Button type="primary" onClick={() => history.push('/workplace')}>
          返回工作台
        </Button>
      }
    />
  );
}
