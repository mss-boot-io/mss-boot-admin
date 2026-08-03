import { Access } from '@/components/MssBoot/Access';
import {
  deleteLanguagesId,
  getLanguages,
  getLanguagesId,
  postLanguages,
  putLanguagesId,
} from '@/services/admin/language';
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
import { FormattedMessage, history, useIntl, Link, useParams } from '@umijs/max';
import { Button, Drawer, List, message, Popconfirm, Typography } from 'antd';
import React, { useRef, useState } from 'react';
import { v4 as uuidv4 } from 'uuid';
import { fieldIntl } from '@/util/fieldIntl';
import MobileLanguageList from './Mobile/LanguageList';

const Language: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const { id } = useParams();
  const [showDetail, setShowDetail] = useState<boolean>(false);
  const [currentRow, setCurrentRow] = useState<API.Language>();
  const { isMobile } = useResponsive();
  const intl = useIntl();
  const formRef = useRef<ProFormInstance>();
  const [editableKeys, setEditableKeys] = useState<React.Key[]>(() => []);

  const columnsTable: ProColumns<API.LanguageDefine>[] = [
    {
      title: fieldIntl(intl, 'id'),
      dataIndex: 'id',
      hideInForm: true,
      hideInTable: true,
    },
    {
      title: fieldIntl(intl, 'group'),
      dataIndex: 'group',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
      width: '30%',
    },
    {
      title: fieldIntl(intl, 'key'),
      dataIndex: 'key',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
      width: '30%',
    },
    {
      title: fieldIntl(intl, 'value'),
      dataIndex: 'value',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
      width: '30%',
    },
    {
      title: <FormattedMessage id="pages.title.option" />,
      valueType: 'option',
      width: 200,
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
            const tableDataSource = formRef.current?.getFieldValue(
              'defines',
            ) as API.LanguageDefine[];
            formRef.current?.setFieldsValue({
              defines: tableDataSource.filter((item) => item.id !== record.id),
            });
          }}
        >
          <FormattedMessage id="pages.title.delete" defaultMessage="Delete" />
        </Button>,
      ],
    },
  ];

  const columns: ProColumns<API.Language>[] = [
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
        1: {
          text: fieldIntl(intl, 'options.enabled'),
          status: '1',
        },
        2: {
          text: fieldIntl(intl, 'options.disabled'),
          status: '2',
        },
      },
    },
    {
      title: fieldIntl(intl, 'language.defines'),
      dataIndex: 'defines',
      hideInTable: true,
      search: false,
      renderText(text) {
        return (
          <List
            bordered
            dataSource={text}
            renderItem={(item: API.LanguageDefine) => (
              <List.Item>
                <Typography.Text mark>
                  {item.group}.{item.id}
                </Typography.Text>
                : {item.value}
              </List.Item>
            )}
          />
        );
      },
      renderFormItem() {
        return (
          <EditableProTable<API.LanguageDefine>
            rowKey="id"
            scroll={{
              x: 960,
            }}
            formRef={formRef}
            // editableFormRef={schema.formRef}
            headerTitle={fieldIntl(intl, 'language.defines')}
            maxLength={1000}
            name="defines"
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
        <Access key="/language/edit">
          <Link to={`/language/${record.id}`}>
            <Button key="edit">
              <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
            </Button>
          </Link>
        </Access>,
        <Access key="/language/delete">
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
              await deleteLanguagesId({ id: record.id! });
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
      await postLanguages(params);
      message.success(
        intl.formatMessage({
          id: 'pages.message.create.success',
          defaultMessage: 'Create successfully!',
        }),
      );
      history.push('/language');
      return;
    }
    await putLanguagesId({ id }, params);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
    history.push('/language');
  };

  return (
    <PageContainer title={indexTitle(id)}>
      {isMobile && !id ? (
        <MobileLanguageList
          request={getLanguages}
          onEdit={(record) => history.push(`/language/${record.id}`)}
          onCreate={() => history.push('/language/create')}
          onDelete={async (record) => {
            await deleteLanguagesId({ id: record.id! });
            message.success(intl.formatMessage({ id: 'pages.message.delete.success' }));
          }}
        />
      ) : (
        <ProTable<API.Language, API.Page>
        headerTitle={intl.formatMessage({
          id: 'pages.language.list.title',
          defaultMessage: 'Language List',
        })}
        actionRef={actionRef}
        formRef={formRef}
        rowKey="id"
        search={false}
        type={id ? 'form' : 'table'}
        onSubmit={id ? onSubmit : undefined}
        toolBarRender={() => [
          <Access key="/language/create">
            <Button type="primary" key="create">
              <Link type="primary" key="primary" to="/language/create">
                <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
              </Link>
            </Button>
          </Access>,
        ]}
        form={
          id && id !== 'create'
            ? {
                request: async () => {
                  const res = await getLanguagesId({ id });
                  return res;
                },
              }
            : undefined
        }
        request={getLanguages}
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
          <ProDescriptions<API.Language>
            column={1}
            title={currentRow?.name}
            request={async () => ({
              data: currentRow || {},
            })}
            params={{
              id: currentRow?.id,
            }}
            columns={columns as ProDescriptionsItemProps<API.Language>[]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default Language;
