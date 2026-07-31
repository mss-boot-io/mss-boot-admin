import { Access } from '@/components/MssBoot/Access';
import {
  deleteFieldsId,
  getFields,
  getFieldsId,
  postFields,
  putFieldsId,
} from '@/services/admin/field';
import { idRender } from '@/util/columnOptions';
import { indexTitle } from '@/util/indexTitle';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  EditableProTable,
  PageContainer,
  ProColumns,
  ProDescriptions,
  ProDescriptionsItemProps,
  ProFormInstance,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, Link, useIntl, useParams } from '@umijs/max';
import { Button, Drawer, message, Popconfirm } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
// @ts-ignore
import { v4 as uuidv4 } from 'uuid';
import { fieldIntl } from '@/util/fieldIntl';
import { useRequest } from 'ahooks';
import { getOptions } from '@/services/admin/option';
import { toOptions } from '@/util/toOptions';

const Field: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const { id, modelID } = useParams();

  const [showDetail, setShowDetail] = useState<boolean>(false);
  const [currentRow, setCurrentRow] = useState<API.Field>();
  const formRef = useRef<ProFormInstance>();
  const [editableKeys, setEditableKeys] = useState<React.Key[]>(() => []);
  const intl = useIntl();
  const { data: options, loading } = useRequest(async () => {
    const res = await getOptions({ pageSize: 1000 });
    return res.data;
  });

  const columnsTable: ProColumns<API.BaseRule>[] = [
    {
      title: fieldIntl(intl, 'id'),
      dataIndex: 'id',
      hideInForm: true,
      hideInTable: true,
    },
    {
      title: fieldIntl(intl, 'required'),
      dataIndex: 'required',
      valueType: 'switch',
    },
    {
      dataIndex: 'message',
      title: fieldIntl(intl, 'message'),
    },
    {
      dataIndex: 'pattern',
      title: fieldIntl(intl, 'pattern'),
    },
    {
      dataIndex: 'type',
      title: fieldIntl(intl, 'type'),
      valueEnum: {
        string: {
          text: fieldIntl(intl, 'options.string'),
          status: 'string',
        },
        number: {
          text: fieldIntl(intl, 'options.number'),
          status: 'number',
        },
        boolean: {
          text: fieldIntl(intl, 'options.boolean'),
          status: 'boolean',
        },
        method: {
          text: fieldIntl(intl, 'options.method'),
          status: 'method',
        },
        regexp: {
          text: fieldIntl(intl, 'options.regexp'),
          status: 'regexp',
        },
        integer: {
          text: fieldIntl(intl, 'options.integer'),
          status: 'integer',
        },
        float: {
          text: fieldIntl(intl, 'options.float'),
          status: 'float',
        },
        array: {
          text: fieldIntl(intl, 'options.array'),
          status: 'array',
        },
        object: {
          text: fieldIntl(intl, 'options.object'),
          status: 'object',
        },
        enum: {
          text: fieldIntl(intl, 'options.enum'),
          status: 'enum',
        },
        date: {
          text: fieldIntl(intl, 'options.date'),
          status: 'date',
        },
        url: {
          text: fieldIntl(intl, 'options.url'),
          status: 'url',
        },
        hex: {
          text: fieldIntl(intl, 'options.hex'),
          status: 'hex',
        },
        email: {
          text: fieldIntl(intl, 'options.email'),
          status: 'email',
        },
      },
    },
    {
      dataIndex: 'min',
      title: fieldIntl(intl, 'min'),
    },
    {
      dataIndex: 'max',
      title: fieldIntl(intl, 'max'),
    },
    {
      dataIndex: 'len',
      title: fieldIntl(intl, 'len'),
    },
    {
      dataIndex: 'warningOnly',
      title: fieldIntl(intl, 'warningOnly'),
    },
    {
      dataIndex: 'whitespace',
      title: fieldIntl(intl, 'whitespace'),
    },
    {
      dataIndex: 'validateTrigger',
      title: fieldIntl(intl, 'validateTrigger'),
      valueEnum: {
        onChange: {
          text: 'onChange',
          status: 'onChange',
        },
        onBlur: {
          text: 'onBlur',
          status: 'onBlur',
        },
        onValidate: {
          text: 'onValidate',
          status: 'onValidate',
        },
      },
    },
    {
      title: <FormattedMessage id="pages.title.option" defaultMessage="Operating" />,
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
            const tableDataSource = formRef.current?.getFieldValue('defines') as API.BaseRule[];
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

  // @ts-ignore
  const columns: ProColumns<API.Field>[] = [
    {
      title: fieldIntl(intl, 'id'),
      dataIndex: 'id',
      hideInForm: true,
      // hideInTable: true,
      render: (dom, entity) => {
        return idRender(dom, entity, setCurrentRow, setShowDetail);
      },
    },
    {
      title: fieldIntl(intl, 'modelID'),
      dataIndex: 'modelID',
      hideInForm: true,
      hideInTable: true,
    },
    {
      title: fieldIntl(intl, 'name'),
      dataIndex: 'name',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
    },
    {
      title: fieldIntl(intl, 'sort'),
      dataIndex: 'sort',
      valueType: 'digit',
    },
    {
      title: fieldIntl(intl, 'jsonTag'),
      dataIndex: 'jsonTag',
    },
    {
      title: fieldIntl(intl, 'label'),
      dataIndex: 'label',
    },
    {
      title: fieldIntl(intl, 'type'),
      dataIndex: 'type',
      valueEnum: {
        string: {
          text: fieldIntl(intl, 'options.string'),
          status: 'string',
        },
        float: {
          text: fieldIntl(intl, 'options.float'),
          status: 'float',
        },
        int: {
          text: fieldIntl(intl, 'options.int'),
          status: 'int',
        },
        uint: {
          text: fieldIntl(intl, 'options.uint'),
          status: 'uint',
        },
        bool: {
          text: fieldIntl(intl, 'options.bool'),
          status: 'bool',
        },
        time: {
          text: fieldIntl(intl, 'options.time'),
          status: 'time',
        },
        bytes: {
          text: fieldIntl(intl, 'options.bytes'),
          status: 'bytes',
        },
      },
    },
    {
      title: fieldIntl(intl, 'formComponent'),
      dataIndex: 'formComponent',
      valueEnum: {
        richText: {
          text: fieldIntl(intl, 'options.richText'),
          status: 'richText',
        },
        input: {
          text: fieldIntl(intl, 'options.input'),
          status: 'input',
        },
        textarea: {
          text: fieldIntl(intl, 'options.textarea'),
          status: 'textarea',
        },
        password: {
          text: fieldIntl(intl, 'options.password'),
          status: 'password',
        },
        number: {
          text: fieldIntl(intl, 'options.number'),
          status: 'number',
        },
        select: {
          text: fieldIntl(intl, 'options.select'),
          status: 'select',
        },
        radio: {
          text: fieldIntl(intl, 'options.radio'),
          status: 'radio',
        },
        checkbox: {
          text: fieldIntl(intl, 'options.checkbox'),
          status: 'checkbox',
        },
        rate: {
          text: fieldIntl(intl, 'options.rate'),
          status: 'rate',
        },
        slider: {
          text: fieldIntl(intl, 'options.slider'),
          status: 'slider',
        },
        switch: {
          text: fieldIntl(intl, 'options.switch'),
          status: 'switch',
        },
        date: {
          text: fieldIntl(intl, 'options.date'),
          status: 'date',
        },
        time: {
          text: fieldIntl(intl, 'options.time'),
          status: 'time',
        },
        datetime: {
          text: fieldIntl(intl, 'options.datetime'),
          status: 'datetime',
        },
        year: {
          text: fieldIntl(intl, 'options.year'),
          status: 'year',
        },
        month: {
          text: fieldIntl(intl, 'options.month'),
          status: 'month',
        },
        week: {
          text: fieldIntl(intl, 'options.week'),
          status: 'week',
        },
        upload: {
          text: fieldIntl(intl, 'options.upload'),
          status: 'upload',
        },
      },
    },
    {
      title: fieldIntl(intl, 'tableComponent'),
      dataIndex: 'tableComponent',
      valueEnum: {
        avatar: {
          text: fieldIntl(intl, 'options.avatar'),
          status: 'avatar',
        },
      },
    },
    {
      title: fieldIntl(intl, 'valueEnum'),
      dataIndex: 'valueEnumName',
      //@ts-ignore
      valueEnum: toOptions(options),
    },
    {
      title: fieldIntl(intl, 'size'),
      dataIndex: 'size',
      valueType: 'digit',
    },
    {
      title: fieldIntl(intl, 'primaryKey'),
      dataIndex: 'primaryKey',
    },
    {
      title: fieldIntl(intl, 'uniqueKey'),
      dataIndex: 'uniqueKey',
    },
    {
      title: fieldIntl(intl, 'index'),
      dataIndex: 'index',
    },
    {
      title: fieldIntl(intl, 'default'),
      dataIndex: 'default',
    },
    {
      title: fieldIntl(intl, 'comment'),
      dataIndex: 'comment',
    },
    {
      title: fieldIntl(intl, 'search'),
      dataIndex: 'search',
      valueEnum: {
        exact: {
          text: fieldIntl(intl, 'options.exact'),
          status: 'exact',
        },
        contains: {
          text: fieldIntl(intl, 'options.contains'),
          status: 'contains',
        },
        gt: {
          text: fieldIntl(intl, 'options.gt'),
          status: 'gt',
        },
        lt: {
          text: fieldIntl(intl, 'options.lt'),
          status: 'lt',
        },
        startswith: {
          text: fieldIntl(intl, 'options.startswith'),
          status: 'startswith',
        },
        endswith: {
          text: fieldIntl(intl, 'options.endswith'),
          status: 'endswith',
        },
        in: {
          text: fieldIntl(intl, 'options.in'),
          status: 'in',
        },
        isnull: {
          text: fieldIntl(intl, 'options.isnull'),
          status: 'isnull',
        },
        order: {
          text: fieldIntl(intl, 'options.order'),
          status: 'order',
        },
      },
    },
    {
      title: fieldIntl(intl, 'notNull'),
      dataIndex: 'notNull',
      valueType: 'switch',
      search: false,
    },
    {
      title: fieldIntl(intl, 'hideInForm'),
      dataIndex: 'hideInForm',
      valueType: 'switch',
      search: false,
    },
    {
      title: fieldIntl(intl, 'hideInTable'),
      dataIndex: 'hideInTable',
      valueType: 'switch',
      search: false,
    },
    {
      title: fieldIntl(intl, 'hideInDescriptions'),
      dataIndex: 'hideInDescriptions',
      valueType: 'switch',
      search: false,
    },
    {
      title: fieldIntl(intl, 'rules'),
      dataIndex: 'rules',
      hideInTable: true,
      search: false,
      renderFormItem() {
        return (
          <EditableProTable<API.BaseRule>
            rowKey="id"
            scroll={{ x: 960 }}
            formRef={formRef}
            headerTitle={fieldIntl(intl, 'rules')}
            maxLength={10}
            name="rules"
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
      title: <FormattedMessage id="pages.title.option" />,
      dataIndex: 'option',
      valueType: 'option',
      hideInDescriptions: true,
      hideInForm: true,
      render: (_, record) => [
        <Access key="/field/edit">
          <Link to={`/field/${modelID}/${record.id}`}>
            <Button key="edit">
              <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
            </Button>
          </Link>
        </Access>,
        <Access key="/field/delete">
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
              await deleteFieldsId({ id: record.id! });
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
    params.modelID = modelID;
    if (id === 'create') {
      await postFields(params);
      message.success(
        intl.formatMessage({
          id: 'pages.message.create.success',
          defaultMessage: 'Create successfully!',
        }),
      );
      history.push(`/field/${modelID}`);
      return;
    }
    await putFieldsId({ id }, params);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
    history.push(`/field/${modelID}`);
  };

  useEffect(() => {
    // request modelID
    if (!modelID) {
      message.error('modelID is empty').then(() => history.push('/model'));
    }
  }, [modelID]);

  return loading ? (
    <></>
  ) : (
    <PageContainer title={indexTitle(id)}>
      <ProTable<API.Field, API.getFieldsParams>
        headerTitle={intl.formatMessage({
          id: 'pages.field.list.title',
          defaultMessage: 'Field List',
        })}
        actionRef={actionRef}
        formRef={formRef}
        rowKey="id"
        search={false}
        type={id ? 'form' : 'table'}
        onSubmit={id ? onSubmit : undefined}
        toolBarRender={() => [
          <Access key="/field/create">
            <Button type="primary" key="create">
              <Link type="primary" key="primary" to={`/field/${modelID}/create`}>
                <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
              </Link>
            </Button>
          </Access>,
        ]}
        form={
          id && id !== 'create'
            ? {
                request: async () => await getFieldsId({ id }),
              }
            : undefined
        }
        request={getFields}
        params={{ modelID }}
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
          <ProDescriptions<API.Field>
            column={1}
            title={currentRow?.name}
            request={async () => ({
              data: currentRow || {},
            })}
            params={{
              id: currentRow?.id,
            }}
            columns={columns as ProDescriptionsItemProps<API.Field>[]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default Field;
