import { history, useIntl } from '@umijs/max';
import { Button, Result } from 'antd';

export default function NotFoundPage() {
  const intl = useIntl();
  return (
    <Result
      status="404"
      title="404"
      subTitle={intl.formatMessage({ id: 'states.notFound' })}
      extra={
        <Button type="primary" onClick={() => history.push('/workplace')}>
          {intl.formatMessage({ id: 'actions.backToWorkplace' })}
        </Button>
      }
    />
  );
}
