import { Access } from '@/components/MssBoot/Access';
import {
  deleteTasksId,
  getTaskOperateId,
  getTasks,
  getTasksId,
  postTasks,
  putTasksId,
} from '@/services/admin/task';
import { idRender } from '@/util/columnOptions';
import { indexTitle } from '@/util/indexTitle';
import { useResponsive } from '@/hooks/useResponsive';
import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns, ProDescriptionsItemProps } from '@ant-design/pro-components';
import {
  PageContainer,
  ProDescriptions,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl, Link, useParams } from '@umijs/max';
import { Button, Drawer, Form, Input, message, Popconfirm, Row, Col, Tag } from 'antd';
import React, { useRef, useState } from 'react';
import { fieldIntl } from '@/util/fieldIntl';
import MobileTaskList from './Mobile/TaskList';

const TaskList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [provider, setProvider] = useState<string>('default');
  const [needMethod, setNeedMethod] = useState<boolean>(false);
  const [needBody, setNeedBody] = useState<boolean>(false);
  const [mustBody, setMustBody] = useState<boolean>(false);
  const [showDetail, setShowDetail] = useState<boolean>(false);
  const [currentRow, setCurrentRow] = useState<API.Role>();
  const { id } = useParams();
  const { isMobile } = useResponsive();
  const intl = useIntl();

  const columns: ProColumns<API.Task>[] = [
    {
      title: fieldIntl(intl, 'id'),
      dataIndex: 'id',
      hideInForm: true,
      render: (dom, entity) => {
        return idRender(dom, entity, setCurrentRow, setShowDetail);
      },
    },
    {
      title: fieldIntl(intl, 'name'),
      dataIndex: 'name',
    },
    {
      title: fieldIntl(intl, 'spec'),
      dataIndex: 'spec',
      hideInTable: true,
      formItemProps: {
        rules: [{ required: true }],
      },
    },
    {
      title: fieldIntl(intl, 'remark'),
      dataIndex: 'remark',
      search: false,
      valueType: 'textarea',
    },
    {
      title: fieldIntl(intl, 'provider'),
      dataIndex: 'provider',
      valueEnum: {
        default: {
          text: fieldIntl(intl, 'options.default'),
          color: 'default',
        },
        k8s: {
          text: fieldIntl(intl, 'options.k8s'),
          color: 'blue',
        },
      },
    },
    {
      title: fieldIntl(intl, 'cluster'),
      dataIndex: 'cluster',
      hideInTable: true,
      hideInForm: provider !== 'k8s',
      formItemProps: {
        rules: [{ required: true }],
      },
    },
    {
      title: fieldIntl(intl, 'namespace'),
      dataIndex: 'namespace',
      hideInTable: true,
      hideInForm: provider !== 'k8s',
      formItemProps: {
        rules: [{ required: true }],
      },
    },
    {
      title: fieldIntl(intl, 'image'),
      dataIndex: 'image',
      hideInForm: provider !== 'k8s',
      hideInTable: true,
    },
    {
      title: fieldIntl(intl, 'protocol'),
      dataIndex: 'protocol',
      hideInTable: true,
      hideInForm: provider !== 'default',
      renderFormItem() {
        return (
          <ProFormSelect
            options={[
              {
                value: 'http',
                label: 'http',
              },
              {
                value: 'https',
                label: 'https',
              },
              {
                value: 'grpc',
                label: 'grpc',
              },
              {
                value: 'grpcs',
                label: 'grpcs',
              },
            ]}
            name="protocol"
          />
        );
      },
      formItemProps: {
        rules: [{ required: true }],
      },
    },
    {
      title: fieldIntl(intl, 'endpoint'),
      dataIndex: 'endpoint',
      hideInTable: true,
      hideInForm: provider !== 'default',
      formItemProps: {
        rules: [{ required: true }],
      },
    },
    {
      title: fieldIntl(intl, 'method'),
      dataIndex: 'method',
      hideInTable: !needMethod,
      hideInForm: provider !== 'default',
      renderFormItem() {
        return (
          <ProFormSelect
            options={[
              {
                value: 'GET',
                label: 'GET',
              },
              {
                value: 'POST',
                label: 'POST',
              },
              {
                value: 'PUT',
                label: 'PUT',
              },
              {
                value: 'DELETE',
                label: 'DELETE',
              },
            ]}
            name="method"
            placeholder={fieldIntl(intl, 'method.placeholder')}
          />
        );
      },
    },
    {
      title: fieldIntl(intl, 'body'),
      dataIndex: 'body',
      hideInTable: true,
      hideInForm: !needBody && provider !== 'default',
      valueType: 'jsonCode',
      formItemProps: {
        rules: [{ required: mustBody }],
      },
    },
    {
      title: fieldIntl(intl, 'command'),
      dataIndex: 'command',
      valueType: 'text',
      hideInTable: true,
      hideInForm: provider === 'default',
      renderFormItem: (_, formItemProps, fieldProps) => {
        return (
          <Form.List name="command">
            {(fields, { add, remove }) => (
              <>
                {fields.map((field) => (
                  <Row key={field.key} gutter={8}>
                    <Col span={16}>
                      <Form.Item
                        {...field}
                        {...formItemProps}
                        name={[field.name]}
                        rules={[{ required: true, message: 'Missing command' }]}
                        style={{ marginBottom: 0 }}
                      >
                        <Input placeholder="Enter an command" {...fieldProps} />
                      </Form.Item>
                    </Col>
                    <Col span={4}>
                      <MinusCircleOutlined
                        onClick={() => remove(field.name)}
                        style={{ margin: '0 8px', cursor: 'pointer' }}
                      />
                    </Col>
                  </Row>
                ))}
                <Form.Item>
                  <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
                    Add Command
                  </Button>
                </Form.Item>
              </>
            )}
          </Form.List>
        );
      },
      render: (_, entity) => {
        return (
          <div>
            {entity.command?.split('|').map((item: string) => (
              <Tag key={item} color="blue">
                {item}
              </Tag>
            ))}
          </div>
        );
      },
    },
    {
      title: fieldIntl(intl, 'args'),
      dataIndex: 'args',
      valueType: 'text',
      hideInTable: true,
      hideInForm: provider === 'default',
      renderFormItem: (_, formItemProps, fieldProps) => {
        return (
          <Form.List name="args">
            {(fields, { add, remove }) => (
              <>
                {fields.map((field) => (
                  <Row key={field.key} gutter={8}>
                    <Col span={16}>
                      <Form.Item
                        {...field}
                        {...formItemProps}
                        name={[field.name]}
                        rules={[{ required: true, message: 'Missing argument' }]}
                        style={{ marginBottom: 0 }}
                      >
                        <Input placeholder="Enter an argument" {...fieldProps} />
                      </Form.Item>
                    </Col>
                    <Col span={4}>
                      <MinusCircleOutlined
                        onClick={() => remove(field.name)}
                        style={{ margin: '0 8px', cursor: 'pointer' }}
                      />
                    </Col>
                  </Row>
                ))}
                <Form.Item>
                  <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
                    Add Argument
                  </Button>
                </Form.Item>
              </>
            )}
          </Form.List>
        );
      },
      render: (_, entity) => {
        return (
          <div>
            {entity.args?.split('|').map((item: string) => (
              <Tag key={item} color="blue">
                {item}
              </Tag>
            ))}
          </div>
        );
      },
    },
    {
      title: fieldIntl(intl, 'status'),
      dataIndex: 'status',
      formItemProps: {
        rules: [{ required: true }],
      },
      valueEnum: {
        enabled: {
          text: fieldIntl(intl, 'options.enabled'),
          color: 'green',
          status: 'enabled',
        },
        disabled: {
          text: fieldIntl(intl, 'options.disabled'),
          color: 'red',
          status: 'disabled',
        },
        locked: {
          text: fieldIntl(intl, 'options.locked'),
          color: 'yellow',
          status: 'locked',
        },
      },
    },
    {
      title: fieldIntl(intl, 'checkedAt'),
      sorter: false,
      dataIndex: 'checkedAt',
      search: false,
      valueType: 'dateTime',
      hideInForm: true,
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
        <Access key="/task/operate">
          <Button
            key="operate"
            onClick={async () => {
              if (record.status === 'enabled') {
                await getTaskOperateId({ id: record.id!, operate: 'stop' });
                message
                  .success(
                    intl.formatMessage({
                      id: 'pages.message.stop.success',
                      defaultMessage: 'Stop successfully!',
                    }),
                  )
                  .then(() => actionRef.current?.reload());
              }
              // @ts-ignore
              if (!record.status || record.status === '' || record.status === 'disabled') {
                await getTaskOperateId({ id: record.id!, operate: 'start' });
                message
                  .success(
                    intl.formatMessage({
                      id: 'pages.message.start.success',
                      defaultMessage: 'Start successfully!',
                    }),
                  )
                  .then(() => actionRef.current?.reload());
                return;
              }
            }}
          >
            {record.status === 'enabled'
              ? intl.formatMessage({ id: 'pages.task.stop.title' })
              : intl.formatMessage({ id: 'pages.task.start.title' })}
          </Button>
        </Access>,
        <Access key="/task/edit">
          <Link to={`/task/${record.id}`}>
            <Button key="edit">
              <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
            </Button>
          </Link>
        </Access>,
        <Access key="/task/delete">
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
              await deleteTasksId({ id: record.id! });
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
    //将args数组, 转换成json字符串
    if (params.args) {
      params.args = JSON.stringify(params.args);
    }
    //将command数组使用|分割
    if (params.command) {
      params.command = JSON.stringify(params.command);
    }
    if (id === 'create') {
      await postTasks(params);
      message.success(
        intl.formatMessage({
          id: 'pages.message.create.success',
          defaultMessage: 'Create successfully!',
        }),
      );
      history.push('/task');
      return;
    }
    await putTasksId({ id }, params);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
    history.push('/task');
  };

  const onValuesChange = (values: API.Task) => {
    if (values.provider !== undefined) {
      setProvider(values.provider);
    }
    if (values.protocol !== undefined) {
      setNeedMethod(values.protocol === 'http' || values.protocol === 'https');
    }
    if (values.method !== undefined) {
      setNeedBody(values.method !== 'GET');
      setMustBody(values.method === 'POST' || values.method === 'PUT');
    }
  };

  return (
    <PageContainer title={indexTitle(id)}>
      {isMobile && !id ? (
        <MobileTaskList
          request={getTasks}
          onEdit={(record) => history.push(`/task/${record.id}`)}
          onCreate={() => history.push('/task/create')}
          onDelete={async (record) => {
            await deleteTasksId({ id: record.id! });
            message.success(intl.formatMessage({ id: 'pages.message.delete.success' }));
          }}
        />
      ) : (
        <ProTable<API.Task, API.Page>
        headerTitle={intl.formatMessage({
          id: 'pages.task.list.title',
          defaultMessage: 'Options List',
        })}
        actionRef={actionRef}
        rowKey="id"
        search={{
          labelWidth: 120,
        }}
        type={id ? 'form' : 'table'}
        onSubmit={id ? onSubmit : undefined}
        toolBarRender={() => [
          <Access key="/task/create">
            <Button type="primary" key="create">
              <Link type="primary" key="primary" to="/task/create">
                <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
              </Link>
            </Button>
          </Access>,
        ]}
        form={
          id && id !== 'create'
            ? {
                request: async () => {
                  const res = await getTasksId({ id });
                  setProvider(res.provider!);
                  setNeedBody(res.method !== 'GET');
                  if (res.args) {
                    res.args = JSON.parse(res.args);
                  }
                  if (res.command) {
                    res.command = JSON.parse(res.command);
                  }
                  return res;
                },
                onValuesChange,
                initialValues: { provider: 'default', status: 'enabled', namespace: 'default' },
              }
            : {
                onValuesChange,
              }
        }
        request={getTasks}
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
          <ProDescriptions<API.Task>
            column={1}
            title={currentRow?.name}
            request={async (params) => {
              // @ts-ignore
              const res = await getTasksId(params);
              res.name = currentRow?.name;
              return {
                data: res,
              };
            }}
            params={{
              id: currentRow?.id,
            }}
            columns={columns as ProDescriptionsItemProps<API.Task>[]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default TaskList;
