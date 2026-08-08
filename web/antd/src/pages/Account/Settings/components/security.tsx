import React, { useState } from 'react';
import { Button, List, message, Skeleton } from 'antd';
import { ModalForm, ProForm, ProFormText } from '@ant-design/pro-components';
import { useIntl, useModel } from '@umijs/max';
import { fieldIntl } from '@/util/fieldIntl';
import { getUserUserInfo, postUserResetPassword } from '@/services/admin/user';
import { useRequest } from 'ahooks';

type SecurityItem = {
  actions?: React.ReactNode[];
  description: React.ReactNode;
  title: React.ReactNode;
};

// const passwordStrength = {
//   strong: <span className="strong">强</span>,
//   medium: <span className="medium">中</span>,
//   weak: <span className="weak">弱 Weak</span>,
// };

const SecurityView: React.FC = () => {
  const intl = useIntl();
  const { initialState, loading: initialStateLoading } = useModel('@@initialState');

  // const [changePassword, setChangePassword] = useState<boolean>(false);
  const [openChangePassword, setOpenChangePassword] = useState<boolean>(false);

  const initialUser = initialState?.currentUser;
  const { data: fetchedUser, loading: userInfoLoading } = useRequest(getUserUserInfo, {
    // Let the app-wide user request finish first; only unauthenticated/partial
    // initial state needs this component-level fallback.
    ready: !initialStateLoading && !initialUser,
  });
  const currentUser = initialUser ?? fetchedUser;
  const loading = initialStateLoading || userInfoLoading;

  const getData = (): SecurityItem[] => [
    {
      title: intl.formatMessage({
        id: 'pages.security.settings.accountPassword',
        defaultMessage: '账户密码',
      }),
      description: intl.formatMessage({
        id: 'pages.security.settings.passwordDescription',
        defaultMessage: '密码存储方式为非对称加密，请妥善保管',
      }),
      actions: [
        <Button key="Modify" type="link" onClick={() => setOpenChangePassword(true)}>
          {intl.formatMessage({
            id: 'pages.security.settings.modify',
            defaultMessage: '修改',
          })}
        </Button>,
      ],
    },
    {
      title: intl.formatMessage({
        id: 'pages.security.settings.phone',
        defaultMessage: '手机号',
      }),
      description: loading ? (
        <Skeleton.Input active size="small" />
      ) : !currentUser?.phone ? (
        intl.formatMessage({
          id: 'pages.security.settings.phoneUnbound',
          defaultMessage: '未绑定手机',
        })
      ) : (
        intl.formatMessage(
          {
            id: 'pages.security.settings.phoneBound',
            defaultMessage: '已绑定手机：{phone}',
          },
          { phone: currentUser?.phone },
        )
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.security.settings.email',
        defaultMessage: '邮箱',
      }),
      description: loading ? (
        <Skeleton.Input active size="small" />
      ) : !currentUser?.email ? (
        intl.formatMessage({
          id: 'pages.security.settings.emailUnbound',
          defaultMessage: '未绑定邮箱',
        })
      ) : (
        intl.formatMessage(
          {
            id: 'pages.security.settings.emailBound',
            defaultMessage: '已绑定邮箱：{email}',
          },
          { email: currentUser?.email },
        )
      ),
    },
  ];

  const data = getData();
  return (
    <>
      <List<SecurityItem>
        itemLayout="horizontal"
        dataSource={data}
        renderItem={(item) => (
          <List.Item actions={item.actions}>
            <List.Item.Meta title={item.title} description={item.description} />
          </List.Item>
        )}
      />
      <ModalForm
        title={intl.formatMessage({
          id: 'pages.security.settings.changePassword',
          defaultMessage: '修改密码',
        })}
        open={openChangePassword}
        onFinish={async (item: API.ResetPasswordRequest) => {
          await postUserResetPassword(item);
          message.success(
            intl.formatMessage({
              id: 'pages.security.settings.changePasswordSuccess',
              defaultMessage: '修改成功',
            }),
          );
          setOpenChangePassword(false);
          return true;
        }}
        onOpenChange={setOpenChangePassword}
      >
        <ProForm.Group>
          <ProFormText.Password
            name="password"
            label={fieldIntl(intl, 'password')}
            placeholder={intl.formatMessage({
              id: 'pages.login.password.placeholder',
            })}
            rules={[
              { required: true },
              { min: 8, message: intl.formatMessage({ id: 'pages.validate.password.min8' }) },
              { max: 20, message: intl.formatMessage({ id: 'pages.validate.password.max20' }) },
              {
                pattern: /[a-zA-Z]/,
                message: intl.formatMessage({ id: 'pages.validate.password.letter' }),
              },
              {
                pattern: /[0-9]/,
                message: intl.formatMessage({ id: 'pages.validate.password.number' }),
              },
            ]}
          />
        </ProForm.Group>
        <ProForm.Group>
          <ProFormText.Password
            name="confirm"
            label={fieldIntl(intl, 'confirm')}
            placeholder={intl.formatMessage({
              id: 'pages.login.password.placeholder',
            })}
            rules={[
              { required: true },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('password') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(
                    new Error(
                      intl.formatMessage({
                        id: 'pages.login.passwordInconsistent',
                        defaultMessage: '两次密码不一致',
                      }),
                    ),
                  );
                },
              }),
            ]}
          />
        </ProForm.Group>
      </ModalForm>
    </>
  );
};

export default SecurityView;
