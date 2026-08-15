import {
  DeleteOutlined,
  EditOutlined,
  EyeOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons';
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
  Table,
  type TableColumnsType,
  Tag,
  Typography,
} from 'antd';
import { useState } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageEmpty, PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import { queryKeys } from '@/shared/query/client';
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

interface LanguageListViewProps {
  canCreate: boolean;
  canDelete: boolean;
  canEdit: boolean;
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

export default function LanguageListView({ canCreate, canDelete, canEdit }: LanguageListViewProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const [form] = Form.useForm<LanguageFilterValues>();
  const [params, setParams] = useState<LanguageListParams>(initialParams);
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

  const columns: TableColumnsType<LanguageSummary> = [
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
          <Col xs={24} sm={12} lg={7}>
            <Form.Item name="name" label={intl.formatMessage({ id: 'language.field.name' })}>
              <Input allowClear maxLength={255} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} lg={5}>
            <Form.Item name="status" label={intl.formatMessage({ id: 'language.field.status' })}>
              <Select
                options={(['all', 'enabled', 'disabled'] as const).map((value) => ({
                  value,
                  label: intl.formatMessage({ id: `language.status.${value}` }),
                }))}
              />
            </Form.Item>
          </Col>
          <Col xs={24} lg={12}>
            <Form.Item>
              <Space wrap>
                <Button htmlType="submit" icon={<SearchOutlined />} type="primary">
                  {intl.formatMessage({ id: 'actions.search' })}
                </Button>
                <Button
                  onClick={() => {
                    form.resetFields();
                    setParams(initialParams);
                  }}
                >
                  {intl.formatMessage({ id: 'actions.reset' })}
                </Button>
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
      <Table<LanguageSummary>
        columns={columns}
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
        rowKey="id"
        scroll={{ x: 720 }}
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
