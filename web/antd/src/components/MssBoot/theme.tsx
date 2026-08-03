import React, { useRef } from 'react';
import { useIntl } from '@@/exports';
import { ProColumns, ProFormInstance, ProTable } from '@ant-design/pro-components';
import { getAppConfigsGroup, putAppConfigsGroup } from '@/services/admin/appConfig';
import { message } from 'antd';
import { fieldIntl } from '@/util/fieldIntl';

const Theme: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const formRef = useRef<ProFormInstance>();

  const columns: ProColumns<any>[] = [
    {
      title: fieldIntl(intl, 'navTheme'),
      dataIndex: 'navTheme',
      valueType: 'select',
      valueEnum: {
        light: 'light',
        realDark: 'realDark',
      },
    },
    {
      title: fieldIntl(intl, 'primaryColor'),
      dataIndex: 'colorPrimary',
      valueType: 'color',
    },
    {
      title: fieldIntl(intl, 'layout'),
      dataIndex: 'layout',
      valueType: 'select',
      initialValue: 'mix',
      valueEnum: {
        side: 'side',
        top: 'top',
        mix: 'mix',
      },
    },
    {
      title: fieldIntl(intl, 'contentWidth'),
      dataIndex: 'contentWidth',
      valueType: 'select',
      initialValue: 'Fluid',
      valueEnum: {
        Fluid: 'Fluid',
        Fixed: 'Fixed',
      },
    },
    {
      title: fieldIntl(intl, 'fixedHeader'),
      dataIndex: 'fixedHeader',
      valueType: 'switch',
    },
    {
      title: fieldIntl(intl, 'fixSiderbar'),
      dataIndex: 'fixSiderbar',
      valueType: 'switch',
    },
    {
      title: fieldIntl(intl, 'pwa'),
      dataIndex: 'pwa',
      valueType: 'switch',
    },
    {
      title: fieldIntl(intl, 'colorWeak'),
      dataIndex: 'colorWeak',
      valueType: 'switch',
    },
  ];

  const onSubmit = async (params: Record<string, any>) => {
    if (params.colorPrimary) {
      params.colorPrimary = params.colorPrimary.toHexString();
    }
    await putAppConfigsGroup({ group: 'theme' }, { data: params });
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
          return await getAppConfigsGroup({ group: 'theme' });
        },
      }}
    />
  );
};

export default Theme;
