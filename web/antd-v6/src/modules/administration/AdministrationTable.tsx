import type { UseQueryResult } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import type { TableColumnsType } from 'antd';
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Grid,
  Input,
  List,
  Row,
  Select,
  Space,
  Table,
  Tag,
} from 'antd';
import type { Dispatch, ReactNode, SetStateAction } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageEmpty, PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import {
  ADMIN_PAGE_SIZES,
  type AdministrationListParams,
  type AdministrationPage,
  type AdministrationStatus,
  type AdministrationStatusFilter,
  isAdminPageSize,
} from './contract';

interface FilterValues {
  name?: string;
  status: AdministrationStatusFilter;
}

interface AdministrationTableProps<T extends { id: string }> {
  columns: TableColumnsType<T>;
  emptyText: string;
  params: AdministrationListParams;
  query: UseQueryResult<AdministrationPage<T>, Error>;
  setParams: Dispatch<SetStateAction<AdministrationListParams>>;
  mobileColumnKeys?: readonly string[];
  toolbar?: ReactNode;
}

function columnKey<T>(column: TableColumnsType<T>[number]): string | undefined {
  if (column.key !== undefined) return String(column.key);
  if (!('dataIndex' in column) || column.dataIndex === undefined) return undefined;
  return Array.isArray(column.dataIndex)
    ? column.dataIndex.map(String).join('.')
    : String(column.dataIndex);
}

function dataIndexValue<T>(record: T, dataIndex: unknown): unknown {
  const path = Array.isArray(dataIndex) ? dataIndex : [dataIndex];
  return path.reduce<unknown>((value, segment) => {
    if (!value || typeof value !== 'object') return undefined;
    return (value as Record<PropertyKey, unknown>)[segment as PropertyKey];
  }, record);
}

function mobileDescriptionItems<T>(
  columns: TableColumnsType<T>,
  keys: readonly string[],
  record: T,
  index: number,
) {
  const selected = new Set(keys);
  return columns.flatMap((column) => {
    const key = columnKey(column);
    if (!key || !selected.has(key) || 'children' in column) return [];
    const value = dataIndexValue(record, 'dataIndex' in column ? column.dataIndex : undefined);
    const children = ('render' in column && typeof column.render === 'function'
      ? column.render(value, record, index)
      : (value as ReactNode) || '—') as unknown as ReactNode;
    return [
      {
        key,
        label: typeof column.title === 'function' ? key : column.title,
        children,
      },
    ];
  });
}

export function AdministrationStatusTag({ status }: { status: AdministrationStatus }) {
  const intl = useIntl();
  const color = status === 'enabled' ? 'green' : status === 'locked' ? 'orange' : 'red';
  return (
    <Tag color={color}>
      {intl.formatMessage({ id: `administration.status.${status || 'unknown'}` })}
    </Tag>
  );
}

export default function AdministrationTable<T extends { id: string }>({
  columns,
  emptyText,
  params,
  query,
  setParams,
  mobileColumnKeys,
  toolbar,
}: AdministrationTableProps<T>) {
  const intl = useIntl();
  const [form] = Form.useForm<FilterValues>();
  const screens = Grid.useBreakpoint();
  const status = getRequestStatus(query.error);

  if (status === 403) {
    return <PageForbidden message={intl.formatMessage({ id: 'administration.forbidden.read' })} />;
  }
  if (query.isPending && !query.data) return <PageLoading rows={8} />;
  if (query.isError && !query.data) {
    return (
      <PageError
        message={getRequestErrorMessage(query.error)}
        onRetry={() => void query.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
        title={intl.formatMessage({ id: 'states.loadError' })}
      />
    );
  }

  return (
    <Space orientation="vertical" size="middle" className="w-full">
      {query.isError ? (
        <Alert
          showIcon
          title={intl.formatMessage({ id: 'administration.refreshFailed' })}
          type="warning"
        />
      ) : null}
      <Form<FilterValues>
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
            <Form.Item name="name" label={intl.formatMessage({ id: 'administration.field.name' })}>
              <Input allowClear maxLength={255} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12} lg={5}>
            <Form.Item
              name="status"
              label={intl.formatMessage({ id: 'administration.field.status' })}
            >
              <Select
                options={(['all', 'enabled', 'disabled', 'locked'] as const).map((value) => ({
                  value,
                  label: intl.formatMessage({ id: `administration.status.${value}` }),
                }))}
              />
            </Form.Item>
          </Col>
          <Col xs={24} lg={12}>
            <Form.Item>
              <Space wrap>
                <Button htmlType="submit" type="primary">
                  {intl.formatMessage({ id: 'actions.search' })}
                </Button>
                <Button
                  onClick={() => {
                    form.resetFields();
                    setParams({ current: 1, pageSize: 20, status: 'all' });
                  }}
                >
                  {intl.formatMessage({ id: 'actions.reset' })}
                </Button>
                <Button loading={query.isFetching} onClick={() => void query.refetch()}>
                  {intl.formatMessage({ id: 'actions.refresh' })}
                </Button>
                {toolbar}
              </Space>
            </Form.Item>
          </Col>
        </Row>
      </Form>
      {screens.md === false && mobileColumnKeys ? (
        <List<T>
          dataSource={query.data?.data ?? []}
          loading={query.isFetching}
          locale={{ emptyText: <PageEmpty description={emptyText} /> }}
          pagination={{
            current: params.current,
            hideOnSinglePage: true,
            pageSize: params.pageSize,
            simple: true,
            total: query.data?.total ?? 0,
            onChange: (current) => setParams((previous) => ({ ...previous, current })),
          }}
          rowKey={(item) => item.id}
          renderItem={(item, index) => (
            <List.Item style={{ paddingInline: 0 }}>
              <Card className="w-full" size="small">
                <Descriptions
                  column={1}
                  items={mobileDescriptionItems(columns, mobileColumnKeys, item, index)}
                  size="small"
                />
              </Card>
            </List.Item>
          )}
        />
      ) : (
        <Table<T>
          columns={columns}
          dataSource={query.data?.data ?? []}
          loading={query.isFetching}
          locale={{ emptyText: <PageEmpty description={emptyText} /> }}
          pagination={{
            current: params.current,
            pageSize: params.pageSize,
            pageSizeOptions: ADMIN_PAGE_SIZES.map(String),
            showSizeChanger: true,
            total: query.data?.total ?? 0,
            onChange: (current, pageSize) =>
              setParams((previous) => ({
                ...previous,
                current,
                pageSize: isAdminPageSize(pageSize) ? pageSize : previous.pageSize,
              })),
          }}
          rowKey="id"
          scroll={{ x: 960 }}
        />
      )}
    </Space>
  );
}
