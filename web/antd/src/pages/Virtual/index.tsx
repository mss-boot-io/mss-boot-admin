import { Access } from '@/components/MssBoot/Access';
import { addOption } from '@/util/addOption';
import { idRender } from '@/util/columnOptions';
import { indexTitle } from '@/util/indexTitle';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProDescriptions,
  ProDescriptionsItemProps,
  ProFormInstance,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, Link, useIntl, useLocation, useParams } from '@umijs/max';
import { useRequest } from 'ahooks';
import { Button, Drawer, message } from 'antd';
import React, { useRef, useState } from 'react';
import {
  createVirtualModel,
  getVirtualDocumentation,
  getVirtualModel,
  listVirtualModels,
  updateVirtualModel,
} from './service/virtual';
import RichTextEditor from '@/components/MssBoot/Editor';

const Virtual: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const actionRef = useRef<ActionType>();
  const { id, key } = useParams();
  const { pathname } = useLocation();
  const [showDetail, setShowDetail] = useState<boolean>(false);
  const [currentRow, setCurrentRow] = useState<API.Model>();
  const formRef = useRef<ProFormInstance>();
  const { data, loading } = useRequest(
    () => {
      return getVirtualDocumentation({ key: key! });
    },
    {
      refreshDeps: [key, id],
    },
  );

  const setFormItemProps = (rules: API.ColumnType[]): ProColumns<{ [key: string]: any }>[] => {
    let columns: ProColumns<{ [key: string]: any }>[] = [];
    rules.forEach((item) => {
      // @ts-ignore
      let column: ProColumns<{ [key: string]: any }> = {
        ...item,
      };
      switch (item.valueType) {
        case 'richText':
          column.renderFormItem = (text, props) => {
            return (
              <RichTextEditor
                {...props}
                defaultValue={props.value}
                onChange={(value) => {
                  let o = {};
                  // @ts-ignore
                  o[text.dataIndex] = value.toHTML();
                  formRef.current?.setFieldsValue(o);
                }}
              />
            );
          };
          break;
      }
      if (item.pk) {
        column.render = (dom, entity) => {
          return idRender(dom, entity, setCurrentRow, setShowDetail);
        };
      }
      if (item.validateRules && item.validateRules.length > 0) {
        // @ts-ignore
        column.formItemProps = () => {
          return {
            rules: item.validateRules,
          };
        };
      }

      columns.push(column);
    });
    return columns;
  };

  const getListPath = (path: string): string => {
    const lastIndex = path.lastIndexOf('/');
    return path.substring(0, lastIndex);
  };

  const onSubmit = async (params: { [key: string]: any }) => {
    if (!id) {
      return;
    }

    if (id === 'create') {
      await createVirtualModel({ key }, params);
      message.success(
        intl.formatMessage({
          id: 'pages.message.create.success',
          defaultMessage: 'Create successfully!',
        }),
      );
      history.push(getListPath(pathname));
      return;
    }
    await updateVirtualModel({ id, key }, params);
    message.success(
      intl.formatMessage({
        id: 'pages.message.edit.success',
        defaultMessage: 'Update successfully!',
      }),
    );
    history.push(getListPath(pathname));
  };

  return loading ? (
    <></>
  ) : (
    <PageContainer title={indexTitle(id)}>
      <ProTable<
        { [key: string]: any },
        // @ts-ignore
        API.listVirtualModelsParams
      >
        headerTitle={intl.formatMessage({
          id: `pages.${data.name}.list.title`,
          defaultMessage: `${data.name} List`,
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
              <Link type="primary" key="primary" to={`${pathname}/create`}>
                <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
              </Link>
            </Button>
          </Access>,
        ]}
        form={
          id && id !== 'create'
            ? {
                request: async () => {
                  const res = await getVirtualModel({ id, key });
                  return res;
                },
              }
            : undefined
        }
        params={{ key }}
        request={listVirtualModels}
        // @ts-ignore
        columns={addOption(intl, pathname, key, actionRef, setFormItemProps(data.columns))}
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
          <ProDescriptions<{ [key: string]: any }>
            column={1}
            title={currentRow?.name}
            request={async () => ({
              data: currentRow || {},
            })}
            params={{
              id: currentRow?.id,
            }}
            columns={
              setFormItemProps(data.columns) as ProDescriptionsItemProps<{ [key: string]: any }>[]
            }
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default Virtual;
