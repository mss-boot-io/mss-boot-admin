import { getRequestErrorMessage, getRequestStatus } from '@mss-admin-core/shared/api/errors';
import {
  PageEmpty,
  PageError,
  PageForbidden,
  PageLoading,
} from '@mss-admin-core/shared/design-system/PageState';
import ResponsiveEntityTable from '@mss-admin-core/shared/design-system/ResponsiveEntityTable';
import type {
  PageRenderField,
  PresentationDensity,
} from '@mss-admin-core/shared/presentation/contract';
import type { UseQueryResult } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import type { TableColumnsType } from 'antd';
import { Alert, Button, Col, Form, Input, Row, Select, Space, Tag } from 'antd';
import {
  type Dispatch,
  type ReactNode,
  type SetStateAction,
  useEffect,
  useRef,
  useState,
} from 'react';
import {
  ADMIN_PAGE_SIZES,
  type AdministrationListParams,
  type AdministrationPage,
  type AdministrationStatus,
  type AdministrationStatusFilter,
  type AdminPageSize,
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
  mobileColumnKeys: readonly string[];
  nameSearch?: PageRenderField | null;
  statusSearch?: PageRenderField | null;
  resetPageSize?: AdminPageSize;
  density?: PresentationDensity;
  searchCollapsedByDefault?: boolean;
  toolbar?: ReactNode;
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
  nameSearch,
  statusSearch,
  resetPageSize = 20,
  density,
  searchCollapsedByDefault = false,
  toolbar,
}: AdministrationTableProps<T>) {
  const intl = useIntl();
  const [form] = Form.useForm<FilterValues>();
  const [searchExpanded, setSearchExpanded] = useState(() => !searchCollapsedByDefault);
  const searchExpansionWasChanged = useRef(false);
  const status = getRequestStatus(query.error);
  const nameSearchField: PageRenderField | null =
    nameSearch === undefined
      ? {
          component: 'input',
          field: 'name',
          label: intl.formatMessage({ id: 'administration.field.name' }),
          order: 10,
        }
      : nameSearch;
  const statusSearchField: PageRenderField | null =
    statusSearch === undefined
      ? {
          component: 'status-filter',
          field: 'status',
          label: intl.formatMessage({ id: 'administration.field.status' }),
          order: 20,
        }
      : statusSearch;

  useEffect(() => {
    if (!searchExpansionWasChanged.current) {
      setSearchExpanded(!searchCollapsedByDefault);
    }
  }, [searchCollapsedByDefault]);

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
          {searchExpanded && nameSearchField ? (
            <Col xs={24} sm={12} lg={7} style={{ order: nameSearchField.order }}>
              <Form.Item name="name" label={nameSearchField.label} extra={nameSearchField.help}>
                <Input allowClear maxLength={255} placeholder={nameSearchField.placeholder} />
              </Form.Item>
            </Col>
          ) : null}
          {searchExpanded && statusSearchField ? (
            <Col xs={24} sm={12} lg={5} style={{ order: statusSearchField.order }}>
              <Form.Item
                name="status"
                label={statusSearchField.label}
                extra={statusSearchField.help}
              >
                <Select
                  options={(['all', 'enabled', 'disabled', 'locked'] as const).map((value) => ({
                    value,
                    label: intl.formatMessage({ id: `administration.status.${value}` }),
                  }))}
                  placeholder={statusSearchField.placeholder}
                />
              </Form.Item>
            </Col>
          ) : null}
          <Col xs={24} lg={12} style={{ order: 10_000 }}>
            <Form.Item>
              <Space wrap>
                {searchExpanded ? (
                  <>
                    <Button htmlType="submit" type="primary">
                      {intl.formatMessage({ id: 'actions.search' })}
                    </Button>
                    <Button
                      onClick={() => {
                        form.resetFields();
                        setParams({ current: 1, pageSize: resetPageSize, status: 'all' });
                      }}
                    >
                      {intl.formatMessage({ id: 'actions.reset' })}
                    </Button>
                  </>
                ) : (
                  <Button
                    onClick={() => {
                      searchExpansionWasChanged.current = true;
                      setSearchExpanded(true);
                    }}
                  >
                    {intl.formatMessage({ id: 'actions.search' })}
                  </Button>
                )}
                <Button loading={query.isFetching} onClick={() => void query.refetch()}>
                  {intl.formatMessage({ id: 'actions.refresh' })}
                </Button>
                {toolbar}
              </Space>
            </Form.Item>
          </Col>
        </Row>
      </Form>
      <ResponsiveEntityTable<T>
        columns={columns}
        dataSource={query.data?.data ?? []}
        loading={query.isFetching}
        locale={{ emptyText: <PageEmpty description={emptyText} /> }}
        mobileColumnKeys={mobileColumnKeys}
        size={density === 'compact' ? 'small' : density}
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
    </Space>
  );
}
