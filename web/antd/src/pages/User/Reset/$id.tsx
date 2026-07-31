import { putUserUserIdPasswordReset } from '@/services/admin/user';
import { PageContainer, ProForm, ProFormInstance, ProFormText } from '@ant-design/pro-components';
import { history, useParams } from '@umijs/max';
import { message } from 'antd';
import { useRef } from 'react';

const PasswordRest: React.FC = () => {
  const { id } = useParams();
  const formRef = useRef<ProFormInstance>();
  const onFinish = async () => {
    // console.log(formRef.current?.getFieldsValue());
    // console.log(id);
    const res = await putUserUserIdPasswordReset(
      { userID: id! },
      formRef.current?.getFieldsValue(),
    );
    if (res) {
      message.success('密码重置成功').then(() => {
        history.push('/users');
      });
    }
  };

  return (
    <PageContainer title="重置密码">
      <ProForm formRef={formRef} omitNil={true} onFinish={onFinish}>
        <ProFormText.Password
          name="password"
          label="密码"
          placeholder="请输入密码"
          rules={[
            { required: true, message: '请输入密码' },
            { min: 8, message: '密码至少8位' },
            { max: 20, message: '密码最多20位' },
            { pattern: /[a-zA-Z]/, message: '密码必须包含字母' },
            { pattern: /[0-9]/, message: '密码必须包含数字' },
          ]}
        />
        <ProFormText.Password
          name="confirm"
          label="确认密码"
          placeholder="请确认密码"
          rules={[
            { required: true, message: '请确认密码' },
            ({ getFieldValue }) => ({
              validator(_, value) {
                if (!value || getFieldValue('password') === value) {
                  return Promise.resolve();
                }
                return Promise.reject(new Error('两次密码不一致'));
              },
            }),
          ]}
        />
      </ProForm>
    </PageContainer>
  );
};

export default PasswordRest;
