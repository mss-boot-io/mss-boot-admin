import { history, useIntl } from '@umijs/max';
import { Button, Result } from 'antd';

export default function ForbiddenPage() {
  const intl = useIntl();
  return (
    <Result
      status="403"
      title="403"
      subTitle={intl.formatMessage({ id: 'states.forbidden' })}
      extra={
        <Button type="primary" onClick={() => history.push('/workplace')}>
          {intl.formatMessage({ id: 'actions.backToWorkplace' })}
        </Button>
      }
    />
  );
}
