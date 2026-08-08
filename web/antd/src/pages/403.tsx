import { history, useIntl } from '@umijs/max';
import { Button, Result } from 'antd';
import React from 'react';

const ForbiddenPage: React.FC = () => {
  const intl = useIntl();
  return (
    <Result
      status="403"
      title="403"
      subTitle={intl.formatMessage({ id: 'pages.403.description' })}
      extra={
        <Button type="primary" onClick={() => history.push('/account/settings')}>
          {intl.formatMessage({ id: 'pages.403.action.account' })}
        </Button>
      }
    />
  );
};

export default ForbiddenPage;
