import { history } from '@umijs/max';
import { Button, Result } from 'antd';

export default function ForbiddenPage() {
  return (
    <Result
      status="403"
      title="403"
      subTitle="你没有访问此页面的权限。"
      extra={
        <Button type="primary" onClick={() => history.push('/workplace')}>
          返回工作台
        </Button>
      }
    />
  );
}
