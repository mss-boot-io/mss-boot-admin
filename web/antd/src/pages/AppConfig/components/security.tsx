import { ProColumns, ProFormInstance, ProTable } from '@ant-design/pro-components';
import React, { useRef } from 'react';
import { useIntl } from '@umijs/max';
import { message } from 'antd';
import { getAppConfigsGroup, putAppConfigsGroup } from '@/services/admin/appConfig';
import { fieldIntl } from '@/util/fieldIntl';
const Security: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const formRef = useRef<ProFormInstance>();

  const [github, setGithub] = React.useState(false);
  const [lark, setLark] = React.useState(false);

  const columns: ProColumns<any>[] = [
    {
      title: fieldIntl(intl, 'registerEnabled'),
      dataIndex: 'registerEnabled',
      valueType: 'switch',
    },
    {
      title: fieldIntl(intl, 'loginByEmailEnabled'),
      dataIndex: 'emailEnabled',
      valueType: 'switch',
    },
    {
      title: fieldIntl(intl, 'githubLoginEnabled'),
      dataIndex: 'githubEnabled',
      valueType: 'switch',
    },
    {
      title: fieldIntl(intl, 'githubClientId'),
      dataIndex: 'githubClientId',
      valueType: 'text',
      formItemProps: () => {
        return github
          ? {
              rules: [{ required: true }],
            }
          : {};
      },
    },
    {
      title: fieldIntl(intl, 'githubClientSecret'),
      dataIndex: 'githubClientSecret',
      valueType: 'password',
      formItemProps: () => {
        return github
          ? {
              rules: [{ required: true }],
            }
          : {};
      },
    },
    {
      title: fieldIntl(intl, 'githubRedirectURI'),
      dataIndex: 'githubRedirectURI',
      valueType: 'text',
      formItemProps: () => {
        return github
          ? {
              rules: [{ required: true }],
            }
          : {};
      },
    },
    {
      title: fieldIntl(intl, 'githubScope'),
      dataIndex: 'githubScope',
      valueType: 'text',
      formItemProps: () => {
        return github
          ? {
              rules: [{ required: true }],
            }
          : {};
      },
    },
    {
      title: fieldIntl(intl, 'githubAllowGroup'),
      dataIndex: 'githubAllowGroup',
      valueType: 'text',
    },
    {
      title: fieldIntl(intl, 'larkLoginEnabled'),
      dataIndex: 'larkEnabled',
      valueType: 'switch',
    },
    {
      title: fieldIntl(intl, 'larkAppId'),
      dataIndex: 'larkAppId',
      hideInForm: !lark,
      valueType: 'text',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
    },
    {
      title: fieldIntl(intl, 'larkAppSecret'),
      dataIndex: 'larkAppSecret',
      hideInForm: !lark,
      valueType: 'password',
    },
    {
      title: fieldIntl(intl, 'larkRedirectURI'),
      dataIndex: 'larkRedirectURI',
      hideInForm: !lark,
      valueType: 'text',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
    },
  ];

  const onSubmit = async (params: Record<string, any>) => {
    params.githubEnabled = github;
    params.larkEnabled = lark;
    await putAppConfigsGroup({ group: 'security' }, { data: params });
    message.success(
      intl.formatMessage({ id: 'pages.message.edit.success', defaultMessage: 'Update Success!' }),
    );
  };

  return (
    <ProTable<any>
      type="form"
      formRef={formRef}
      columns={columns}
      onSubmit={onSubmit}
      form={{
        request: async () => {
          const res = await getAppConfigsGroup({ group: 'security' });
          setGithub(res.githubEnabled);
          setLark(res.larkEnabled);
          return res;
        },
        onValuesChange: (values) => {
          if (values.larkEnabled !== undefined) {
            setLark(values.larkEnabled);
          }
          if (values.githubEnabled !== undefined) {
            setGithub(values.githubEnabled);
          }
        },
      }}
    />
  );
};

export default Security;
