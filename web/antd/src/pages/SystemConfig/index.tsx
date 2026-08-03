import { Access } from '@/components/MssBoot/Access';
import {
  deleteSystemConfigsId,
  getSystemConfigs,
  getSystemConfigsId,
  postSystemConfigs,
  putSystemConfigsId,
} from '@/services/admin/systemConfig';
import { idRender } from '@/util/columnOptions';
import { indexTitle } from '@/util/indexTitle';
import { PlusOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns, ProDescriptionsItemProps } from '@ant-design/pro-components';
import { PageContainer, ProDescriptions, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, history, Link, useParams } from '@umijs/max';
import { Button, Drawer, message, Popconfirm } from 'antd';
import React, { useRef, useState } from 'react';
import { useIntl } from '@@/exports';
import { fieldIntl } from '@/util/fieldIntl';

const SystemConfig: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const { id } = useParams();
  const [showDetail, setShowDetail] = useState<boolean>(false);
  const [currentRow, setCurrentRow] = useState<API.SystemConfig>();
  const intl = useIntl();

  const columns: ProColumns<API.SystemConfig>[] = [
    {
      title: fieldIntl(intl, 'id'),
      dataIndex: 'id',
      hideInForm: true,
      search: false,
      render: (dom, entity) => {
        return idRender(dom, entity, setCurrentRow, setShowDetail);
      },
    },
    {
      title: fieldIntl(intl, 'name'),
      dataIndex: 'name',
    },
    {
      title: fieldIntl(intl, 'ext'),
      dataIndex: 'ext',
      search: false,
      valueEnum: {
        yaml: {
          text: 'yaml',
          status: 'yaml',
        },
        json: {
          text: 'json',
          status: 'json',
        },
      },
    },
    {
      title: fieldIntl(intl, 'content'),
      dataIndex: 'content',
      search: false,
      hideInTable: true,
      valueType: 'code',
    },
    {
      title: fieldIntl(intl, 'remark'),
      dataIndex: 'remark',
      search: false,
      valueType: 'textarea',
    },
    {
      title: fieldIntl(intl, 'createdAt'),
      sorter: true,
      dataIndex: 'createdAt',
      search: false,
      hideInForm: true,
      valueType: 'dateTime',
    },
    {
      title: fieldIntl(intl, 'updatedAt'),
      sorter: true,
      dataIndex: 'updatedAt',
      search: false,
      hideInForm: true,
      valueType: 'dateTime',
    },
    {
      title: <FormattedMessage id="pages.title.option" />,
      dataIndex: 'option',
      valueType: 'option',
      hideInDescriptions: true,
      hideInForm: true,
      render: (_, record) => [
        <Access key="/system-config/edit">
          <Link to={`/system-config/${record.id}`}>
            <Button key="edit">
              <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
            </Button>
          </Link>
        </Access>,
        <Access key="/system-config/delete">
          <Popconfirm
            key="delete"
            title={intl.formatMessage({
              id: 'pages.title.delete.confirm',
              defaultMessage: 'Confirm Delete',
            })}
            description={intl.formatMessage({
              id: 'pages.description.delete.confirm',
              defaultMessage: 'Are you sure to delete this record?',
            })}
            disabled={record.isBuiltIn}
            onConfirm={async () => {
              await deleteSystemConfigsId({ id: record.id! });
              message
                .success(
                  intl.formatMessage({
                    id: 'pages.message.delete.success',
                    defaultMessage: 'Delete successfully!',
                  }),
                )
                .then(() => actionRef.current?.reload());
            }}
            okText={intl.formatMessage({ id: 'pages.title.ok', defaultMessage: 'OK' })}
            cancelText={intl.formatMessage({ id: 'pages.title.cancel', defaultMessage: 'Cancel' })}
          >
            <Button disabled={record.isBuiltIn} key="delete.button">
              <FormattedMessage id="pages.title.delete" defaultMessage="Delete" />
            </Button>
          </Popconfirm>
        </Access>,
      ],
    },
  ];

  const onSubmit = async (params: any) => {
    if (!id) {
      return;
    }
    if (id === 'create') {
      await postSystemConfigs(params);
      message.success(
        intl.formatMessage({
          id: 'pages.message.create.success',
          defaultMessage: 'Create successfully!',
        }),
      );
      history.push('/system-config');
      return;
    }
    await putSystemConfigsId({ id }, params);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
    history.push('/system-config');
  };

  return (
    <PageContainer title={indexTitle(id)}>
      <ProTable<API.SystemConfig, API.Page>
        headerTitle={intl.formatMessage({
          id: 'pages.system.config.list.title',
          defaultMessage: 'System Config List',
        })}
        actionRef={actionRef}
        rowKey="id"
        search={false}
        type={id ? 'form' : 'table'}
        onSubmit={id ? onSubmit : undefined}
        toolBarRender={() => [
          <Access key="/system-config/create">
            <Button type="primary" key="create">
              <Link type="primary" key="primary" to="/system-config/create">
                <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
              </Link>
            </Button>
          </Access>,
        ]}
        form={
          id && id !== 'create'
            ? {
                request: async () => {
                  const res = await getSystemConfigsId({ id });
                  return res;
                },
              }
            : undefined
        }
        request={getSystemConfigs}
        columns={columns}
      />

      <Drawer
        width={600}
        open={showDetail}
        onClose={() => {
          setCurrentRow(undefined);
          setShowDetail(false);
        }}
        closable={false}
      >
        {currentRow?.name && (
          <ProDescriptions<API.SystemConfig>
            column={1}
            title={currentRow?.name}
            request={async () => ({
              data: currentRow || {},
            })}
            params={{
              id: currentRow?.id,
            }}
            columns={columns as ProDescriptionsItemProps<API.SystemConfig>[]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default SystemConfig;
