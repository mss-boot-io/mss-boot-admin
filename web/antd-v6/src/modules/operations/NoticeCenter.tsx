import CheckOutlined from '@ant-design/icons/CheckOutlined';
import EyeOutlined from '@ant-design/icons/EyeOutlined';
import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import SearchOutlined from '@ant-design/icons/SearchOutlined';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl, useSearchParams } from '@umijs/max';
import {
  Alert,
  App,
  Badge,
  Button,
  Col,
  Descriptions,
  Drawer,
  Form,
  Grid,
  Input,
  Row,
  Select,
  Space,
  type TableColumnsType,
  Tag,
  Typography,
} from 'antd';
import { useEffect, useState } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageEmpty, PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import ResponsiveEntityTable from '@/shared/design-system/ResponsiveEntityTable';
import { queryKeys } from '@/shared/query/client';
import { operationsAPI } from './api';
import {
  isOperationsPageSize,
  type NoticeListParams,
  type NoticeStatus,
  type NoticeSummary,
  type NoticeType,
  OPERATIONS_PAGE_SIZES,
} from './contract';
import { useNotice, useNoticePage } from './query';

interface NoticeCenterProps {
  canMarkRead: boolean;
}

interface NoticeFilterValues {
  title?: string;
  status: NoticeStatus | 'all';
  type: NoticeType | 'all';
}

const noticeTypes = ['notification', 'message', 'event', 'mail'] as const;

function isNoticeType(value: string | null): value is NoticeType {
  return noticeTypes.some((candidate) => candidate === value);
}

export default function NoticeCenter({ canMarkRead }: NoticeCenterProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const screens = Grid.useBreakpoint();
  const [searchParams] = useSearchParams();
  const requestedType = searchParams.get('type');
  const routeType: NoticeType | 'all' = isNoticeType(requestedType) ? requestedType : 'all';
  const initialParams: NoticeListParams = {
    current: 1,
    pageSize: 20,
    status: 'all',
    type: routeType,
  };
  const [form] = Form.useForm<NoticeFilterValues>();
  const [params, setParams] = useState<NoticeListParams>(initialParams);
  const [detailID, setDetailID] = useState<string>();
  const notices = useNoticePage(params);
  const detail = useNotice(detailID);
  const listStatus = getRequestStatus(notices.error);
  const isFilterFormMounted =
    listStatus !== 403 &&
    !(notices.isPending && !notices.data) &&
    !(notices.isError && !notices.data);
  const markRead = useMutation({
    mutationFn: (id: string) => operationsAPI.notices.markRead(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.notices });
      void message.success(intl.formatMessage({ id: 'notice.read.success' }));
    },
  });

  useEffect(() => {
    setParams((current) =>
      current.type === routeType ? current : { ...current, current: 1, type: routeType },
    );
    if (isFilterFormMounted) form.setFieldValue('type', routeType);
  }, [form, isFilterFormMounted, routeType]);

  const formatDate = (value?: string) =>
    value
      ? new Intl.DateTimeFormat(intl.locale || 'zh-CN', {
          dateStyle: 'medium',
          timeStyle: 'short',
        }).format(new Date(value))
      : '—';

  const columns: TableColumnsType<NoticeSummary> = [
    {
      title: intl.formatMessage({ id: 'notice.field.title' }),
      dataIndex: 'title',
      render: (_: unknown, notice) => (
        <Space>
          <Badge status={notice.read ? 'default' : 'processing'} />
          <Button className="px-0" type="link" onClick={() => setDetailID(notice.id)}>
            {notice.title}
          </Button>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({ id: 'notice.field.type' }),
      dataIndex: 'type',
      width: 130,
      render: (value: NoticeType) => (
        <Tag color={value === 'message' ? 'blue' : value === 'event' ? 'gold' : 'default'}>
          {intl.formatMessage({ id: `notice.type.${value}` })}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({ id: 'notice.field.status' }),
      dataIndex: 'status',
      responsive: ['md'],
      width: 130,
      render: (value: NoticeStatus) =>
        value ? (
          <Tag color={value === 'urgent' ? 'red' : value === 'todo' ? 'gold' : 'blue'}>
            {intl.formatMessage({ id: `notice.status.${value}` })}
          </Tag>
        ) : (
          '—'
        ),
    },
    {
      title: intl.formatMessage({ id: 'notice.field.description' }),
      dataIndex: 'description',
      responsive: ['lg'],
      ellipsis: true,
      render: (value: string) => value || '—',
    },
    {
      title: intl.formatMessage({ id: 'notice.field.sentAt' }),
      dataIndex: 'createdAt',
      responsive: ['md'],
      width: 190,
      render: (value: string) => formatDate(value),
    },
    {
      title: intl.formatMessage({ id: 'notice.field.actions' }),
      key: 'actions',
      width: canMarkRead ? 220 : 110,
      fixed: screens.md ? 'right' : undefined,
      render: (_: unknown, notice) => (
        <Space size="small" wrap>
          <Button
            icon={<EyeOutlined />}
            size="small"
            type="link"
            onClick={() => setDetailID(notice.id)}
          >
            {intl.formatMessage({ id: 'actions.view' })}
          </Button>
          {canMarkRead && !notice.read ? (
            <Button
              icon={<CheckOutlined />}
              loading={markRead.isPending && markRead.variables === notice.id}
              size="small"
              type="link"
              onClick={() => markRead.mutate(notice.id)}
            >
              {intl.formatMessage({ id: 'notice.read.action' })}
            </Button>
          ) : null}
        </Space>
      ),
    },
  ];

  if (listStatus === 403) {
    return <PageForbidden message={intl.formatMessage({ id: 'notice.forbidden.read' })} />;
  }
  if (notices.isPending && !notices.data) return <PageLoading rows={8} />;
  if (notices.isError && !notices.data) {
    return (
      <PageError
        message={getRequestErrorMessage(notices.error)}
        onRetry={() => void notices.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
        title={intl.formatMessage({ id: 'states.loadError' })}
      />
    );
  }

  return (
    <Space orientation="vertical" size="middle" className="w-full">
      {notices.isError ? (
        <Alert
          showIcon
          title={intl.formatMessage({ id: 'operations.refreshFailed' })}
          type="warning"
        />
      ) : null}
      {markRead.isError ? (
        <Alert
          closable
          description={getRequestErrorMessage(markRead.error)}
          showIcon
          title={intl.formatMessage({ id: 'notice.read.failed' })}
          type="error"
          onClose={() => markRead.reset()}
        />
      ) : null}
      <Form<NoticeFilterValues>
        form={form}
        initialValues={{ status: 'all', type: routeType }}
        layout="vertical"
        onFinish={(values) =>
          setParams((current) => ({
            ...current,
            current: 1,
            title: values.title?.trim() || undefined,
            status: values.status,
            type: values.type,
          }))
        }
      >
        <Row align="bottom" gutter={16}>
          <Col xs={24} sm={12} lg={7}>
            <Form.Item name="title" label={intl.formatMessage({ id: 'notice.field.title' })}>
              <Input allowClear maxLength={255} />
            </Form.Item>
          </Col>
          <Col xs={12} sm={6} lg={4}>
            <Form.Item name="type" label={intl.formatMessage({ id: 'notice.field.type' })}>
              <Select
                options={(['all', ...noticeTypes] as const).map((value) => ({
                  value,
                  label: intl.formatMessage({ id: `notice.type.${value}` }),
                }))}
              />
            </Form.Item>
          </Col>
          <Col xs={12} sm={6} lg={4}>
            <Form.Item name="status" label={intl.formatMessage({ id: 'notice.field.status' })}>
              <Select
                options={(['all', 'urgent', 'doing', 'processing', 'todo'] as const).map(
                  (value) => ({
                    value,
                    label: intl.formatMessage({ id: `notice.status.${value}` }),
                  }),
                )}
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
                  loading={notices.isFetching}
                  onClick={() => void notices.refetch()}
                >
                  {intl.formatMessage({ id: 'actions.refresh' })}
                </Button>
                {canMarkRead ? (
                  <Button
                    icon={<CheckOutlined />}
                    loading={markRead.isPending && markRead.variables === 'all'}
                    onClick={() => markRead.mutate('all')}
                  >
                    {intl.formatMessage({ id: 'notice.read.all' })}
                  </Button>
                ) : null}
              </Space>
            </Form.Item>
          </Col>
        </Row>
      </Form>
      <ResponsiveEntityTable<NoticeSummary>
        columns={columns}
        dataSource={notices.data?.data ?? []}
        loading={notices.isFetching}
        locale={{
          emptyText: <PageEmpty description={intl.formatMessage({ id: 'notice.empty' })} />,
        }}
        pagination={{
          current: params.current,
          pageSize: params.pageSize,
          pageSizeOptions: OPERATIONS_PAGE_SIZES.map(String),
          showSizeChanger: true,
          total: notices.data?.total ?? 0,
          onChange: (current, pageSize) =>
            setParams((previous) => ({
              ...previous,
              current,
              pageSize: isOperationsPageSize(pageSize) ? pageSize : previous.pageSize,
            })),
        }}
        mobileColumnKeys={['title', 'type', 'status', 'description', 'sentAt', 'actions']}
        rowKey="id"
        scroll={{ x: 880 }}
      />
      <Drawer
        destroyOnHidden
        open={Boolean(detailID)}
        size={screens.md ? 640 : '100%'}
        title={detail.data?.title ?? intl.formatMessage({ id: 'notice.detail.title' })}
        onClose={() => setDetailID(undefined)}
      >
        {detail.isPending ? <PageLoading rows={6} /> : null}
        {detail.isError ? (
          <PageError
            message={getRequestErrorMessage(detail.error)}
            onRetry={() => void detail.refetch()}
            retryLabel={intl.formatMessage({ id: 'actions.retry' })}
          />
        ) : null}
        {detail.data ? (
          <Space orientation="vertical" size="large" className="w-full">
            <Descriptions
              column={1}
              items={[
                {
                  key: 'type',
                  label: intl.formatMessage({ id: 'notice.field.type' }),
                  children: intl.formatMessage({ id: `notice.type.${detail.data.type}` }),
                },
                {
                  key: 'status',
                  label: intl.formatMessage({ id: 'notice.field.status' }),
                  children: detail.data.status
                    ? intl.formatMessage({ id: `notice.status.${detail.data.status}` })
                    : '—',
                },
                {
                  key: 'createdAt',
                  label: intl.formatMessage({ id: 'notice.field.sentAt' }),
                  children: formatDate(detail.data.createdAt),
                },
                {
                  key: 'read',
                  label: intl.formatMessage({ id: 'notice.field.readState' }),
                  children: intl.formatMessage({
                    id: detail.data.read ? 'notice.read.read' : 'notice.read.unread',
                  }),
                },
              ]}
            />
            <Typography.Paragraph className="whitespace-pre-wrap">
              {detail.data.description || '—'}
            </Typography.Paragraph>
            {canMarkRead && !detail.data.read ? (
              <Button
                icon={<CheckOutlined />}
                loading={markRead.isPending && markRead.variables === detail.data.id}
                type="primary"
                onClick={() => markRead.mutate(detail.data.id)}
              >
                {intl.formatMessage({ id: 'notice.read.action' })}
              </Button>
            ) : null}
          </Space>
        ) : null}
      </Drawer>
    </Space>
  );
}
