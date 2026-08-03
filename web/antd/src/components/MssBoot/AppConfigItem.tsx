import { Form, Input, Switch } from 'antd';
import React from 'react';

export type AppConfigItemProps = {
  dataIndex: string;
  required: boolean;
  defaultChecked?: boolean;
};

const AppConfigItem: React.FC<AppConfigItemProps> = (props) => {
  return (
    <>
      <Form.Item name={[props.dataIndex, 'value']} noStyle rules={[{ required: props.required }]}>
        <Input style={{ width: '95%' }} />
      </Form.Item>
      <Form.Item name={[props.dataIndex, 'auth']} noStyle>
        <Switch
          checkedChildren="认证"
          unCheckedChildren="开放"
          defaultChecked={props.defaultChecked}
        />
      </Form.Item>
    </>
  );
};

export default AppConfigItem;
