import DeleteOutlined from '@ant-design/icons/DeleteOutlined';
import EditOutlined from '@ant-design/icons/EditOutlined';
import EyeOutlined from '@ant-design/icons/EyeOutlined';
import LockOutlined from '@ant-design/icons/LockOutlined';
import PlusOutlined from '@ant-design/icons/PlusOutlined';
import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import SearchOutlined from '@ant-design/icons/SearchOutlined';
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
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageEmpty, PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import ResponsiveEntityTable from '@/shared/design-system/ResponsiveEntityTable';
import { queryKeys } from '@/shared/query/client';
import { optionAPI } from './api';
import {
  isOptionPageSize,
  OPTION_PAGE_SIZES,
  type OptionListParams,
  type OptionStatusFilter,
  type OptionSummary,
} from './contract';
import OptionDetailDrawer from './OptionDetailDrawer';
import { useOptionPage } from './query';

interface OptionListViewProps {
  canCreate: boolean;
  canDelete: boolean;
  canEdit: boolean;
}

interface OptionFilterValues {
  category?: string;
  name?: string;
  status: OptionStatusFilter;
}

const initialParams: OptionListParams = {
  current: 1,
  pageSize: 20,
  status: 'all',
};

export default function OptionListView({ canCreate, canDelete, canEdit }: OptionListViewProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const [form] = Form.useForm<OptionFilterValues>();
  const [params, setParams] = useState<OptionListParams>(initialParams);
  const [detailID, setDetailID] = useState<string>();
  const options = useOptionPage(params);
  const remove = useMutation({
    mutationFn: (option: OptionSummary) => optionAPI.remove(option),
    onSuccess: async () => {
      if ((options.data?.data.length ?? 0) === 1 && params.current > 1) {
        setParams((current) => ({ ...current, current: current.current - 1 }));
      }
      await client.invalidateQueries({ queryKey: queryKeys.options });
      void message.success(intl.formatMessage({ id: 'option.delete.success' }));
    },
    onError: async (error) => {
      if (getRequestStatus(error) === 412) {
        await client.invalidateQueries({ queryKey: queryKeys.options });
      }
    },
  });
  const listStatus = getRequestStatus(options.error);
  const removeStatus = getRequestStatus(remove.error);

  if (listStatus === 403 || removeStatus === 403) {
    return <PageForbidden message={intl.formatMessage({ id: 'option.forbidden.read' })} />;
  }

  const formatDate = (value: string) =>
    new Intl.DateTimeFormat(intl.locale || 'zh-CN', {
      dateStyle: 'short',
      timeStyle: 'medium',
    }).format(new Date(value));

  const renderStatus = (row: OptionSummary) => (
    <Tag color={row.status === 'enabled' ? 'green' : 'red'}>
      {intl.formatMessage({ id: `option.status.${row.status}` })}
    </Tag>
  );

  const renderActions = (row: OptionSummary) => (
    <Space size="small" wrap>
      <Button icon={<EyeOutlined />} size="small" type="link" onClick={() => setDetailID(row.id)}>
        {intl.formatMessage({ id: 'actions.view' })}
      </Button>
      {canEdit ? (
        <Button
          icon={<EditOutlined />}
          size="small"
          type="link"
          onClick={() => history.push(`/option/${encodeURIComponent(row.id)}`)}
        >
          {intl.formatMessage({ id: 'actions.edit' })}
        </Button>
      ) : null}
      {row.builtIn ? (
        <Tag icon={<LockOutlined />}>{intl.formatMessage({ id: 'option.builtIn' })}</Tag>
      ) : canDelete ? (
        <Popconfirm
          description={intl.formatMessage({ id: 'option.delete.description' })}
          title={intl.formatMessage({ id: 'option.delete.confirm' })}
          onConfirm={() => remove.mutate(row)}
        >
          <Button
            danger
            disabled={remove.isPending && remove.variables?.id !== row.id}
            icon={<DeleteOutlined />}
            loading={remove.isPending && remove.variables?.id === row.id}
            size="small"
            type="link"
          >
            {intl.formatMessage({ id: 'actions.delete' })}
          </Button>
        </Popconfirm>
      ) : null}
    </Space>
  );

  const columns: TableColumnsType<OptionSummary> = [
    {
      title: intl.formatMessage({ id: 'option.field.name' }),
      dataIndex: 'name',
      render: (_, row) => (
        <Button type="link" className="px-0" onClick={() => setDetailID(row.id)}>
          {row.name}
        </Button>
      ),
    },
    {
      title: intl.formatMessage({ id: 'option.field.displayName' }),
      dataIndex: 'displayName',
      ellipsis: true,
      responsive: ['md'],
      render: (_, row) => row.displayName || '—',
    },
    {
      title: intl.formatMessage({ id: 'option.field.category' }),
      dataIndex: 'category',
      width: 140,
      render: (_, row) => <Typography.Text code>{row.category}</Typography.Text>,
    },
    {
      title: intl.formatMessage({ id: 'option.field.status' }),
      dataIndex: 'status',
      width: 110,
      render: (_, row) => renderStatus(row),
    },
    {
      title: intl.formatMessage({ id: 'option.field.version' }),
      dataIndex: 'version',
      responsive: ['lg'],
      width: 90,
    },
    {
      title: intl.formatMessage({ id: 'option.field.updatedAt' }),
      dataIndex: 'updatedAt',
      responsive: ['xl'],
      width: 190,
      render: (_, row) => formatDate(row.updatedAt),
    },
    {
      title: intl.formatMessage({ id: 'option.field.actions' }),
      key: 'actions',
      width: 270,
      render: (_, row) => renderActions(row),
    },
  ];

  if (options.isPending && !options.data) return <PageLoading rows={8} />;
  if (options.isError && (listStatus === 401 || !options.data)) {
    return (
      <PageError
        message={getRequestErrorMessage(options.error)}
        onRetry={() => void options.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
        title={intl.formatMessage({ id: 'states.loadError' })}
      />
    );
  }

  return (
    <Space orientation="vertical" size="middle" className="w-full">
      {options.isError ? (
        <Alert showIcon title={intl.formatMessage({ id: 'option.refreshFailed' })} type="warning" />
      ) : null}
      {remove.isError ? (
        <Alert
          closable
          description={
            removeStatus === 412
              ? intl.formatMessage({ id: 'option.delete.conflict' })
              : getRequestErrorMessage(remove.error)
          }
          showIcon
          title={intl.formatMessage({ id: 'option.delete.failed' })}
          type={removeStatus === 412 ? 'warning' : 'error'}
          onClose={() => remove.reset()}
        />
      ) : null}
      <Form<OptionFilterValues>
        form={form}
        initialValues={{ status: 'all' }}
        layout="vertical"
        onFinish={(values) =>
          setParams((current) => ({
            ...current,
            current: 1,
            category: values.category?.trim() || undefined,
            name: values.name?.trim() || undefined,
            status: values.status,
          }))
        }
      >
        <Row align="bottom" gutter={16}>
          <Col xs={24} sm={12} lg={6}>
            <Form.Item name="name" label={intl.formatMessage({ id: 'option.field.name' })}>
              <Input allowClear maxLength={255} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} lg={5}>
            <Form.Item name="category" label={intl.formatMessage({ id: 'option.field.category' })}>
              <Input allowClear maxLength={50} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} lg={4}>
            <Form.Item name="status" label={intl.formatMessage({ id: 'option.field.status' })}>
              <Select
                options={(['all', 'enabled', 'disabled'] as const).map((value) => ({
                  value,
                  label: intl.formatMessage({ id: `option.status.${value}` }),
                }))}
              />
            </Form.Item>
          </Col>
          <Col xs={24} lg={9}>
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
                  loading={options.isFetching}
                  onClick={() => void options.refetch()}
                >
                  {intl.formatMessage({ id: 'actions.refresh' })}
                </Button>
                {canCreate ? (
                  <Button
                    icon={<PlusOutlined />}
                    type="primary"
                    onClick={() => history.push('/option/create')}
                  >
                    {intl.formatMessage({ id: 'option.create.action' })}
                  </Button>
                ) : null}
              </Space>
            </Form.Item>
          </Col>
        </Row>
      </Form>
      <ResponsiveEntityTable<OptionSummary>
        columns={columns}
        dataSource={options.data?.data ?? []}
        loading={options.isFetching}
        locale={{
          emptyText: <PageEmpty description={intl.formatMessage({ id: 'option.empty' })} />,
        }}
        mobileColumnKeys={['name', 'displayName', 'category', 'status', 'version', 'actions']}
        pagination={{
          current: params.current,
          pageSize: params.pageSize,
          pageSizeOptions: OPTION_PAGE_SIZES.map(String),
          showSizeChanger: true,
          total: options.data?.total ?? 0,
          onChange: (current, pageSize) =>
            setParams((previous) => ({
              ...previous,
              current,
              pageSize: isOptionPageSize(pageSize) ? pageSize : previous.pageSize,
            })),
        }}
        rowKey="id"
        scroll={{ x: 980 }}
      />
      <Typography.Text type="secondary">
        {intl.formatMessage({ id: 'option.summaryNotice' })}
      </Typography.Text>
      <OptionDetailDrawer
        id={detailID}
        open={Boolean(detailID)}
        onClose={() => setDetailID(undefined)}
      />
    </Space>
  );
}
