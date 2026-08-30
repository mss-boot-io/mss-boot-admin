import DeleteOutlined from '@ant-design/icons/DeleteOutlined';
import EditOutlined from '@ant-design/icons/EditOutlined';
import EyeOutlined from '@ant-design/icons/EyeOutlined';
import PlusOutlined from '@ant-design/icons/PlusOutlined';
import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import SearchOutlined from '@ant-design/icons/SearchOutlined';
import { getRequestErrorMessage, getRequestStatus } from '@mss-admin-core/shared/api/errors';
import {
  PageEmpty,
  PageError,
  PageForbidden,
  PageLoading,
} from '@mss-admin-core/shared/design-system/PageState';
import ResponsiveEntityTable from '@mss-admin-core/shared/design-system/ResponsiveEntityTable';
import type { PagePresentationRuntime } from '@mss-admin-core/shared/presentation/runtime';
import {
  resolveTablePresentation,
  usePresentationPageParams,
  usePresentationSearchExpansion,
} from '@mss-admin-core/shared/presentation/table';
import { queryKeys } from '@mss-admin-core/shared/query/client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { history, useIntl } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Col,
  Form,
  Input,
  Popconfirm,
  Row,
  Select,
  Space,
  type TableColumnsType,
  Tag,
  Typography,
} from 'antd';
import { useState } from 'react';
import { languageAPI } from './api';
import {
  isLanguagePageSize,
  LANGUAGE_PAGE_SIZES,
  type LanguageListParams,
  type LanguageStatusFilter,
  type LanguageSummary,
} from './contract';
import LanguageDetailDrawer from './LanguageDetailDrawer';
import { useLanguagePage } from './query';
import {
  languagePresentationListComponents,
  languagePresentationMobileFields,
  languagePresentationSearchComponents,
} from './tablePresentation';

interface LanguageListViewProps {
  canCreate: boolean;
  canDelete: boolean;
  canEdit: boolean;
  presentationRuntime: PagePresentationRuntime;
}

interface LanguageFilterValues {
  name?: string;
  status: LanguageStatusFilter;
}

const initialParams: LanguageListParams = {
  current: 1,
  pageSize: 20,
  status: 'all',
};

export default function LanguageListView({
  canCreate,
  canDelete,
  canEdit,
  presentationRuntime,
}: LanguageListViewProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const [form] = Form.useForm<LanguageFilterValues>();
  const presentation = presentationRuntime.model;
  const configuredPageSize = isLanguagePageSize(presentation.list.pageSize)
    ? presentation.list.pageSize
    : initialParams.pageSize;
  const [params, setParams] = usePresentationPageParams(initialParams, configuredPageSize);
  const searchPresentation = usePresentationSearchExpansion(presentation.search.collapsedByDefault);
  const [detailID, setDetailID] = useState<string>();
  const languages = useLanguagePage(params);
  const remove = useMutation({
    mutationFn: (id: string) => languageAPI.remove(id),
    onSuccess: async () => {
      if ((languages.data?.data.length ?? 0) === 1 && params.current > 1) {
        setParams((current) => ({ ...current, current: current.current - 1 }));
      }
      await client.invalidateQueries({ queryKey: queryKeys.languages });
      void message.success(intl.formatMessage({ id: 'language.delete.success' }));
      void message.info(intl.formatMessage({ id: 'language.runtime.reloadNotice' }));
    },
  });
  const listStatus = getRequestStatus(languages.error);
  const removeStatus = getRequestStatus(remove.error);

  if (listStatus === 403 || removeStatus === 403) {
    return <PageForbidden message={intl.formatMessage({ id: 'language.forbidden.read' })} />;
  }

  const formatDate = (value: string) =>
    new Intl.DateTimeFormat(intl.locale || 'zh-CN', {
      dateStyle: 'short',
      timeStyle: 'medium',
    }).format(new Date(value));

  const compiledColumns: TableColumnsType<LanguageSummary> = [
    {
      title: intl.formatMessage({ id: 'language.field.name' }),
      dataIndex: 'name',
      render: (_, row) => (
        <Button type="link" className="px-0" onClick={() => setDetailID(row.id)}>
          {row.name}
        </Button>
      ),
    },
    {
      title: intl.formatMessage({ id: 'language.field.status' }),
      dataIndex: 'status',
      width: 110,
      render: (_, row) => (
        <Tag color={row.status === 'enabled' ? 'green' : 'red'}>
          {intl.formatMessage({ id: `language.status.${row.status}` })}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({ id: 'language.field.remark' }),
      dataIndex: 'remark',
      ellipsis: true,
      responsive: ['md'],
      render: (_, row) => row.remark || '—',
    },
    {
      title: intl.formatMessage({ id: 'language.field.updatedAt' }),
      dataIndex: 'updatedAt',
      responsive: ['lg'],
      width: 190,
      render: (_, row) => formatDate(row.updatedAt),
    },
    {
      title: intl.formatMessage({ id: 'language.field.actions' }),
      key: 'actions',
      width: 250,
      render: (_, row) => (
        <Space size="small" wrap>
          <Button
            icon={<EyeOutlined />}
            size="small"
            type="link"
            onClick={() => setDetailID(row.id)}
          >
            {intl.formatMessage({ id: 'actions.view' })}
          </Button>
          {canEdit ? (
            <Button
              icon={<EditOutlined />}
              size="small"
              type="link"
              onClick={() => history.push(`/language/${encodeURIComponent(row.id)}`)}
            >
              {intl.formatMessage({ id: 'actions.edit' })}
            </Button>
          ) : null}
          {canDelete ? (
            <Popconfirm
              description={intl.formatMessage({ id: 'language.delete.description' })}
              title={intl.formatMessage({ id: 'language.delete.confirm' })}
              onConfirm={() => remove.mutate(row.id)}
            >
              <Button
                danger
                disabled={remove.isPending && remove.variables !== row.id}
                icon={<DeleteOutlined />}
                loading={remove.isPending && remove.variables === row.id}
                size="small"
                type="link"
              >
                {intl.formatMessage({ id: 'actions.delete' })}
              </Button>
            </Popconfirm>
          ) : null}
        </Space>
      ),
    },
  ];
  const tablePresentation = resolveTablePresentation({
    compiledColumns,
    fallbackPageSize: initialParams.pageSize,
    isPageSize: isLanguagePageSize,
    listComponents: languagePresentationListComponents,
    mobileColumnKeys: [...languagePresentationMobileFields, 'actions'],
    model: presentation,
    protectedColumnKeys: ['actions'],
    searchComponents: languagePresentationSearchComponents,
  });
  const nameSearch = tablePresentation.searchFields.get('name');
  const statusSearch = tablePresentation.searchFields.get('status');

  if (languages.isPending && !languages.data) return <PageLoading rows={8} />;
  if (languages.isError && (listStatus === 401 || !languages.data)) {
    return (
      <PageError
        message={getRequestErrorMessage(languages.error)}
        onRetry={() => void languages.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
        title={intl.formatMessage({ id: 'states.loadError' })}
      />
    );
  }

  return (
    <Space orientation="vertical" size="middle" className="w-full">
      {languages.isError ? (
        <Alert
          showIcon
          title={intl.formatMessage({ id: 'language.refreshFailed' })}
          type="warning"
        />
      ) : null}
      {remove.isError ? (
        <Alert
          closable
          description={getRequestErrorMessage(remove.error)}
          showIcon
          title={intl.formatMessage({ id: 'language.delete.failed' })}
          type="error"
          onClose={() => remove.reset()}
        />
      ) : null}
      <Form<LanguageFilterValues>
        form={form}
        initialValues={{ status: 'all' }}
        layout="vertical"
        onFinish={(values) =>
          setParams((current) => ({
            ...current,
            current: 1,
            name: values.name?.trim() || undefined,
            status: values.status,
          }))
        }
      >
        <Row align="bottom" gutter={16}>
          {searchPresentation.expanded && nameSearch ? (
            <Col xs={24} sm={12} lg={7} style={{ order: nameSearch.order }}>
              <Form.Item name="name" label={nameSearch.label} extra={nameSearch.help}>
                <Input allowClear maxLength={255} placeholder={nameSearch.placeholder} />
              </Form.Item>
            </Col>
          ) : null}
          {searchPresentation.expanded && statusSearch ? (
            <Col xs={24} sm={12} lg={5} style={{ order: statusSearch.order }}>
              <Form.Item name="status" label={statusSearch.label} extra={statusSearch.help}>
                <Select
                  placeholder={statusSearch.placeholder}
                  options={(['all', 'enabled', 'disabled'] as const).map((value) => ({
                    value,
                    label: intl.formatMessage({ id: `language.status.${value}` }),
                  }))}
                />
              </Form.Item>
            </Col>
          ) : null}
          <Col xs={24} lg={12} style={{ order: 10_000 }}>
            <Form.Item>
              <Space wrap>
                {searchPresentation.expanded ? (
                  <>
                    <Button htmlType="submit" icon={<SearchOutlined />} type="primary">
                      {intl.formatMessage({ id: 'actions.search' })}
                    </Button>
                    <Button
                      onClick={() => {
                        form.resetFields();
                        setParams({ ...initialParams, pageSize: tablePresentation.pageSize });
                      }}
                    >
                      {intl.formatMessage({ id: 'actions.reset' })}
                    </Button>
                  </>
                ) : (
                  <Button icon={<SearchOutlined />} onClick={searchPresentation.expand}>
                    {intl.formatMessage({ id: 'actions.search' })}
                  </Button>
                )}
                <Button
                  icon={<ReloadOutlined />}
                  loading={languages.isFetching}
                  onClick={() => void languages.refetch()}
                >
                  {intl.formatMessage({ id: 'actions.refresh' })}
                </Button>
                {canCreate ? (
                  <Button
                    icon={<PlusOutlined />}
                    type="primary"
                    onClick={() => history.push('/language/create')}
                  >
                    {intl.formatMessage({ id: 'language.create.action' })}
                  </Button>
                ) : null}
              </Space>
            </Form.Item>
          </Col>
        </Row>
      </Form>
      <ResponsiveEntityTable<LanguageSummary>
        columns={tablePresentation.columns}
        dataSource={languages.data?.data ?? []}
        loading={languages.isFetching}
        locale={{
          emptyText: <PageEmpty description={intl.formatMessage({ id: 'language.empty' })} />,
        }}
        pagination={{
          current: params.current,
          pageSize: params.pageSize,
          pageSizeOptions: LANGUAGE_PAGE_SIZES.map(String),
          showSizeChanger: true,
          total: languages.data?.total ?? 0,
          onChange: (current, pageSize) =>
            setParams((previous) => ({
              ...previous,
              current,
              pageSize: isLanguagePageSize(pageSize) ? pageSize : previous.pageSize,
            })),
        }}
        mobileColumnKeys={tablePresentation.mobileColumnKeys}
        rowKey="id"
        scroll={{ x: 720 }}
        size={tablePresentation.density === 'compact' ? 'small' : tablePresentation.density}
      />
      <Typography.Text type="secondary">
        {intl.formatMessage({ id: 'language.summaryNotice' })}
      </Typography.Text>
      <LanguageDetailDrawer
        id={detailID}
        open={Boolean(detailID)}
        onClose={() => setDetailID(undefined)}
      />
    </Space>
  );
}
