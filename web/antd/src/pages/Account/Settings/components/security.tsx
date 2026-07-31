import React, { useState } from 'react';
import { List, message } from 'antd';
import { ModalForm, ProForm, ProFormText } from '@ant-design/pro-components';
import { useIntl } from '@@/exports';
import { fieldIntl } from '@/util/fieldIntl';
import { getUserUserInfo, postUserResetPassword } from '@/services/admin/user';
import { useRequest } from 'ahooks';

type Unpacked<T> = T extends (infer U)[] ? U : T;

// const passwordStrength = {
//   strong: <span className="strong">强</span>,
//   medium: <span className="medium">中</span>,
//   weak: <span className="weak">弱 Weak</span>,
// };

const SecurityView: React.FC = () => {
  const intl = useIntl();

  // const [changePassword, setChangePassword] = useState<boolean>(false);
  const [openChangePassword, setOpenChangePassword] = useState<boolean>(false);

  const { data: currentUser, loading } = useRequest(async () => {
    const res = await getUserUserInfo();
    if (res) {
      return res;
    }
    return {};
  });

  const getData = () => [
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
        <a key="Modify" onClick={() => setOpenChangePassword(true)}>
          {intl.formatMessage({
            id: 'pages.security.settings.modify',
            defaultMessage: '修改',
          })}
        </a>,
      ],
    },
    {
      title: intl.formatMessage({
        id: 'pages.security.settings.phone',
        defaultMessage: '手机号',
      }),
      description:
        loading || !currentUser?.phone
          ? intl.formatMessage({
              id: 'pages.security.settings.phoneUnbound',
              defaultMessage: '未绑定手机',
            })
          : intl.formatMessage(
              {
                id: 'pages.security.settings.phoneBound',
                defaultMessage: '已绑定手机：{phone}',
              },
              { phone: currentUser?.phone },
            ),
      actions: [
        <a key="Modify">
          {currentUser?.phone
            ? intl.formatMessage({
                id: 'pages.security.settings.modify',
                defaultMessage: '修改',
              })
            : intl.formatMessage({
                id: 'pages.security.settings.bind',
                defaultMessage: '绑定',
              })}
        </a>,
      ],
    },
    {
      title: intl.formatMessage({
        id: 'pages.security.settings.email',
        defaultMessage: '邮箱',
      }),
      description:
        loading || !currentUser?.email
          ? intl.formatMessage({
              id: 'pages.security.settings.emailUnbound',
              defaultMessage: '未绑定邮箱',
            })
          : intl.formatMessage(
              {
                id: 'pages.security.settings.emailBound',
                defaultMessage: '已绑定邮箱：{email}',
              },
              { email: currentUser?.email },
            ),
      actions: [
        <a key="Modify">
          {currentUser?.email
            ? intl.formatMessage({
                id: 'pages.security.settings.modify',
                defaultMessage: '修改',
              })
            : intl.formatMessage({
                id: 'pages.security.settings.bind',
                defaultMessage: '绑定',
              })}
        </a>,
      ],
    },
  ];

  const data = getData();
  return (
    <>
      <List<Unpacked<typeof data>>
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
