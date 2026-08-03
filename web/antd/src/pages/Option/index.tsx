import { Access } from '@/components/MssBoot/Access';
import {
  deleteOptionsId,
  getOptions,
  getOptionsId,
  postOptions,
  putOptionsId,
} from '@/services/admin/option';
import { idRender } from '@/util/columnOptions';
import { indexTitle } from '@/util/indexTitle';
import { useResponsive } from '@/hooks/useResponsive';
import { PlusOutlined } from '@ant-design/icons';
import type {
  ActionType,
  ProColumns,
  ProDescriptionsItemProps,
  ProFormInstance,
} from '@ant-design/pro-components';
import {
  EditableProTable,
  PageContainer,
  ProDescriptions,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl, history, Link, useParams } from '@umijs/max';
import { Button, Drawer, List, message, Popconfirm, Typography } from 'antd';
import React, { useRef, useState } from 'react';
import { v4 as uuidv4 } from 'uuid';
import { fieldIntl } from '@/util/fieldIntl';
import MobileOptionList from './Mobile/OptionList';

const Option: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const { id } = useParams();
  const [showDetail, setShowDetail] = useState<boolean>(false);
  const [currentRow, setCurrentRow] = useState<API.Option>();
  const formRef = useRef<ProFormInstance>();
  const [editableKeys, setEditableKeys] = useState<React.Key[]>(() => []);
  const { isMobile } = useResponsive();
  const intl = useIntl();

  const columnsTable: ProColumns<API.OptionItem>[] = [
    {
      title: fieldIntl(intl, 'id'),
      dataIndex: 'id',
      hideInForm: true,
      hideInTable: true,
    },
    {
      title: fieldIntl(intl, 'label'),
      dataIndex: 'label',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
    },
    {
      title: fieldIntl(intl, 'key'),
      dataIndex: 'key',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
    },
    {
      title: fieldIntl(intl, 'value'),
      dataIndex: 'value',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
    },
    {
      title: fieldIntl(intl, 'color'),
      dataIndex: 'color',
      valueEnum: {
        red: {
          text: fieldIntl(intl, 'options.red'),
          status: 'red',
          color: 'red',
        },
        green: {
          text: fieldIntl(intl, 'options.green'),
          status: 'green',
          color: 'green',
        },
        yellow: {
          text: fieldIntl(intl, 'options.yellow'),
          status: 'yellow',
          color: 'yellow',
        },
        orange: {
          text: fieldIntl(intl, 'options.orange'),
          status: 'orange',
          color: 'orange',
        },
        blue: {
          text: fieldIntl(intl, 'options.blue'),
          status: 'blue',
          color: 'blue',
        },
        purple: {
          text: fieldIntl(intl, 'options.purple'),
          status: 'purple',
          color: 'purple',
        },
        cyan: {
          text: fieldIntl(intl, 'options.cyan'),
          status: 'cyan',
          color: 'cyan',
        },
        volcano: {
          text: fieldIntl(intl, 'options.volcano'),
          status: 'volcano',
          color: 'volcano',
        },
      },
    },
    {
      title: <FormattedMessage id="pages.title.option" />,
      valueType: 'option',
      render: (text, record, _, action) => [
        <Button
          key="editable"
          onClick={() => {
            action?.startEditable?.(record.id!);
          }}
        >
          <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
        </Button>,
        <Button
          key="delete"
          onClick={async () => {
            const tableDataSource = formRef.current?.getFieldValue('items') as API.OptionItem[];
            formRef.current?.setFieldsValue({
              items: tableDataSource.filter((item) => item.id !== record.id),
            });
          }}
        >
          <FormattedMessage id="pages.title.delete" defaultMessage="Delete" />
        </Button>,
      ],
    },
  ];

  const columns: ProColumns<API.Option>[] = [
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
      title: fieldIntl(intl, 'status'),
      dataIndex: 'status',
      valueEnum: {
        enabled: {
          text: fieldIntl(intl, 'options.enabled'),
          status: 'enabled',
        },
        disabled: {
          text: fieldIntl(intl, 'options.disabled'),
          status: 'disabled',
        },
      },
    },
    {
      title: <FormattedMessage id="pages.title.option" />,
      dataIndex: 'items',
      hideInTable: true,
      search: false,
      renderText(text) {
        return (
          <List
            bordered
            dataSource={text}
            renderItem={(item: API.OptionItem) => (
              <List.Item>
                <Typography.Text mark>
                  {item.label}[{item.key}]
                </Typography.Text>
                : {item.value}
              </List.Item>
            )}
          />
        );
      },
      renderFormItem() {
        return (
          <EditableProTable<API.OptionItem>
            rowKey="id"
            scroll={{
              x: 960,
            }}
            formRef={formRef}
            // editableFormRef={schema.formRef}
            headerTitle={intl.formatMessage({ id: 'pages.option.dictionary.list' })}
            maxLength={1000}
            name="items"
            controlled={false}
            // @ts-ignore
            recordCreatorProps={{
              position: 'bottom' as 'top',
              // @ts-ignore
              record: () => ({
                id: uuidv4().replace(/-/g, ''),
              }),
            }}
            columns={columnsTable}
            editable={{
              type: 'multiple',
              editableKeys,
              onChange: setEditableKeys,
              actionRender: (row, config, defaultDom) => {
                return [defaultDom.save, defaultDom.delete ?? defaultDom.cancel];
              },
            }}
          />
        );
      },
    },
    {
      title: fieldIntl(intl, 'remark'),
      search: false,
      dataIndex: 'remark',
    },
    {
      title: fieldIntl(intl, 'updatedAt'),
      sorter: true,
      dataIndex: 'updatedAt',
      search: false,
      valueType: 'dateTime',
      hideInForm: true,
    },
    {
      title: <FormattedMessage id="pages.title.option" />,
      dataIndex: 'option',
      valueType: 'option',
      hideInDescriptions: true,
      hideInForm: true,
      render: (_, record) => [
        <Access key="/option/edit">
          <Link to={`/option/${record.id}`}>
            <Button key="edit">
              <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
            </Button>
          </Link>
        </Access>,
        <Access key="/option/delete">
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
            onConfirm={async () => {
              await deleteOptionsId({ id: record.id! });
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
            <Button key="delete.button">
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
      await postOptions(params);
      message.success(
        intl.formatMessage({
          id: 'pages.message.create.success',
          defaultMessage: 'Create successfully!',
        }),
      );
      history.push('/option');
      return;
    }
    await putOptionsId({ id }, params);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
    history.push('/option');
  };

  return (
    <PageContainer title={indexTitle(id)}>
      {isMobile && !id ? (
        <MobileOptionList
          request={getOptions}
          onEdit={(record) => history.push(`/option/${record.id}`)}
          onCreate={() => history.push('/option/create')}
          onDelete={async (record) => {
            await deleteOptionsId({ id: record.id! });
            message.success(intl.formatMessage({ id: 'pages.message.delete.success' }));
          }}
        />
      ) : (
        <ProTable<API.Option, API.getOptionsParams>
        headerTitle={intl.formatMessage({
          id: 'pages.options.list.title',
          defaultMessage: 'Options List',
        })}
        actionRef={actionRef}
        formRef={formRef}
        rowKey="id"
        search={false}
        type={id ? 'form' : 'table'}
        onSubmit={id ? onSubmit : undefined}
        toolBarRender={() => [
          <Access key="/option/create">
            <Button type="primary" key="create">
              <Link type="primary" key="primary" to="/option/create">
                <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
              </Link>
            </Button>
          </Access>,
        ]}
        form={
          id && id !== 'create'
            ? {
                request: async () => {
                  const res = await getOptionsId({ id });
                  return res;
                },
              }
            : undefined
        }
        request={getOptions}
        columns={columns}
        />
      )}

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
          <ProDescriptions<API.Option>
            column={1}
            title={currentRow?.name}
            request={async () => ({
              data: currentRow || {},
            })}
            params={{
              id: currentRow?.id,
            }}
            columns={columns as ProDescriptionsItemProps<API.Option>[]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default Option;
