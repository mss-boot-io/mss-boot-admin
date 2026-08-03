import { Access } from '@/components/MssBoot/Access';
import { getMenus } from '@/services/admin/menu';
import {
  deleteModelsId,
  getModels,
  getModelsId,
  postModels,
  putModelGenerateData,
  putModelsId,
} from '@/services/admin/model';
import { idRender } from '@/util/columnOptions';
import { indexTitle } from '@/util/indexTitle';
import { menuTransferTree } from '@/util/menuTransferTree';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  DrawerForm,
  PageContainer,
  ProColumns,
  ProDescriptions,
  ProDescriptionsItemProps,
  ProFormInstance,
  ProFormText,
  ProFormTreeSelect,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, Link, useIntl, useParams } from '@umijs/max';
import { Button, Drawer, Form, message, Popconfirm } from 'antd';
import React, { useRef, useState } from 'react';
import { fieldIntl } from '@/util/fieldIntl';

const Model: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const actionRef = useRef<ActionType>();
  const { id } = useParams();
  const [showDetail, setShowDetail] = useState<boolean>(false);
  const [currentRow, setCurrentRow] = useState<API.Model>();
  const formRef = useRef<ProFormInstance>();
  const [openSelectMenu, setOpenSelectMenu] = useState<boolean>(false);
  const [generateDataForm] = Form.useForm<{ id: string; menuParentID: string }>();

  const columns: ProColumns<API.Model>[] = [
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
      title: fieldIntl(intl, 'name'),
      dataIndex: 'name',
      formItemProps: () => {
        return {
          rules: [{ required: true }],
        };
      },
    },
    {
      title: fieldIntl(intl, 'table'),
      dataIndex: 'table',
    },
    {
      title: fieldIntl(intl, 'path'),
      dataIndex: 'path',
    },
    {
      title: fieldIntl(intl, 'description'),
      dataIndex: 'description',
    },
    {
      title: fieldIntl(intl, 'hardDeleted'),
      dataIndex: 'hardDeleted',
      valueType: 'switch',
    },
    {
      title: fieldIntl(intl, 'auth'),
      dataIndex: 'auth',
      valueType: 'switch',
    },
    {
      title: <FormattedMessage id="pages.title.option" />,
      dataIndex: 'option',
      valueType: 'option',
      hideInDescriptions: true,
      hideInForm: true,
      render: (_, record) => [
        <Access key="/model/edit">
          <Link to={`/model/${record.id}`}>
            <Button key="edit">
              <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
            </Button>
          </Link>
        </Access>,
        <Access key="/model/field">
          <Link to={`/field/${record.id}`}>
            <Button key="field">
              <FormattedMessage id="pages.model.field.title" defaultMessage="Field" />
            </Button>
          </Link>
        </Access>,
        <Access key="/model/generate-data">
          <Button
            onClick={async () => {
              setCurrentRow(record);
              setOpenSelectMenu(true);
            }}
            disabled={record.generatedData}
          >
            <FormattedMessage id="pages.model.generate.data.title" defaultMessage="Generate Data" />
          </Button>
        </Access>,
        <Access key="/model/delete">
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
              await deleteModelsId({ id: record.id! });
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
      await postModels(params);
      message.success(
        intl.formatMessage({
          id: 'pages.message.create.success',
          defaultMessage: 'Create successfully!',
        }),
      );
      history.push('/model');
      return;
    }
    await putModelsId({ id }, params);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
    history.push('/model');
  };

  const onOpenChange = async (e: boolean) => {
    if (e) {
      return;
    }
    setOpenSelectMenu(e);
  };

  return (
    <PageContainer title={indexTitle(id)}>
      <ProTable<API.Model, API.getModelsParams>
        headerTitle={intl.formatMessage({
          id: 'pages.model.list.title',
          defaultMessage: 'Model List',
        })}
        actionRef={actionRef}
        formRef={formRef}
        rowKey="id"
        search={false}
        type={id ? 'form' : 'table'}
        onSubmit={id ? onSubmit : undefined}
        toolBarRender={() => [
          <Access key="/model/create">
            <Button type="primary" key="create">
              <Link type="primary" key="primary" to="/model/create">
                <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
              </Link>
            </Button>
          </Access>,
        ]}
        form={
          id && id !== 'create'
            ? {
                request: async () => {
                  return await getModelsId({ id });
                },
              }
            : undefined
        }
        request={getModels}
        params={{ preloads: ['Fields'] }}
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
          <ProDescriptions<API.Model>
            column={1}
            title={currentRow?.name}
            request={async () => ({
              data: currentRow || {},
            })}
            params={{
              id: currentRow?.id,
            }}
            columns={columns as ProDescriptionsItemProps<API.Model>[]}
          />
        )}
      </Drawer>
      <DrawerForm<API.ModelGenerateDataRequest>
        title={intl.formatMessage({ id: 'pages.model.parent.select' })}
        width={600}
        form={generateDataForm}
        open={openSelectMenu}
        onOpenChange={onOpenChange}
        onFinish={async (e) => {
          await putModelGenerateData(e);
          message
            .success(intl.formatMessage({ id: 'pages.model.generate.data.success' }))
            .then(() => actionRef.current?.reload());
          setOpenSelectMenu(false);
        }}
      >
        <ProFormText name="id" hidden initialValue={currentRow?.id} />
        <ProFormTreeSelect
          name="menuParentID"
          allowClear
          width="xl"
          placeholder={intl.formatMessage({ id: 'pages.model.parent.placeholder' })}
          request={async () => {
            const res = await getMenus({ pageSize: 1000 });
            // @ts-ignore
            return menuTransferTree(intl, res.data);
          }}
        />
      </DrawerForm>
    </PageContainer>
  );
};

export default Model;
