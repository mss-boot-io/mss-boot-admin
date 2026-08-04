import { Access } from '@/components/MssBoot/Access';
import RichTextEditor from '@/components/MssBoot/Editor';
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
    const columns: ProColumns<{ [key: string]: any }>[] = [];
    rules.forEach((item) => {
      // @ts-ignore
      const column: ProColumns<{ [key: string]: any }> = {
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
                  const fields: Record<string, string> = {};
                  // @ts-ignore
                  fields[text.dataIndex] = value;
                  formRef.current?.setFieldsValue(fields);
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

  const load = async (params: { [key: string]: any }) => {
    params.key = key;
    const list = await listVirtualModels(params);
    return {
      ...list,
      total: list.count,
    };
  };

  const columns = setFormItemProps((data?.form?.columns || []) as API.ColumnType[]);

  return (
    <PageContainer title={indexTitle(intl, data?.model?.displayName)}>
      <ProTable<API.Model, API.PageParams>
        headerTitle={intl.formatMessage({
          id: 'pages.searchTable.title',
          defaultMessage: 'Query Form',
        })}
        actionRef={actionRef}
        formRef={formRef}
        rowKey="id"
        loading={loading}
        search={{
          labelWidth: 120,
        }}
        toolBarRender={() => [
          <Access key="create" accessible={data?.operations?.includes('create')} fallback={null}>
            <Link to={`${pathname}/create`}>
              <Button type="primary">
                <PlusOutlined />
                <FormattedMessage id="pages.searchTable.new" defaultMessage="New" />
              </Button>
            </Link>
          </Access>,
        ]}
        request={load}
        columns={addOption(columns, actionRef, setCurrentRow, setShowDetail)}
        onSubmit={onSubmit}
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
        {currentRow?.id && (
          <ProDescriptions<API.Model>
            column={2}
            title={currentRow?.name}
            request={async () => ({
              data: await getVirtualModel({ id: currentRow?.id, key }),
            })}
            params={{
              id: currentRow?.id,
            }}
            columns={data?.form?.columns as ProDescriptionsItemProps<API.Model>[]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default Virtual;
