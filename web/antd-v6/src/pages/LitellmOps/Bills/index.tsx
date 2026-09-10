import { useIntl } from '@umijs/max';
import {
  Alert,
  Card,
  Col,
  DatePicker,
  Input,
  Row,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
} from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useState } from 'react';
import { useGatewayModels } from '@/modules/litellmops/operations-query';
import { useBillsPage } from '@/modules/litellmops/query';
import {
  type BillsParams,
  formatDateTime,
  formatInt,
  formatUsd,
  type SpendLog,
} from '@/modules/litellmops/types';
import { OpsQueryState } from '@/modules/litellmops/ui';

export default function LitellmOpsBillsPage() {
  const intl = useIntl();
  const t = (id: string) => intl.formatMessage({ id });
  const [params, setParams] = useState<BillsParams>({ page: 1, page_size: 20 });
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null]>([null, null]);
  const bills = useBillsPage(params);
  const gatewayModels = useGatewayModels();
  const { data } = bills;
  const modelOptions = (gatewayModels.data?.items ?? [])
    .map((model) => model.name)
    .filter((model, index, models) => Boolean(model) && models.indexOf(model) === index)
    .sort()
    .map((value) => ({ value, label: value }));

  const columns = [
    {
      title: t('litellmops.bills.time'),
      dataIndex: 'start_time',
      render: (v: string) => formatDateTime(v),
    },
    {
      title: t('litellmops.gateway.model'),
      dataIndex: 'model',
      render: (v: string) => <code>{v || '—'}</code>,
    },
    {
      title: t('litellmops.common.status'),
      dataIndex: 'status',
      render: (v: string) =>
        v === 'success' ? (
          <Tag color="green">{t('litellmops.bills.success')}</Tag>
        ) : (
          <Tag color="red">{v || t('litellmops.bills.failure')}</Tag>
        ),
    },
    {
      title: t('litellmops.bills.promptTokens'),
      dataIndex: 'prompt_tokens',
      render: (v: number) => formatInt(v),
    },
    {
      title: t('litellmops.bills.completionTokens'),
      dataIndex: 'completion_tokens',
      render: (v: number) => formatInt(v),
    },
    {
      title: t('litellmops.bills.totalTokens'),
      dataIndex: 'total_tokens',
      render: (v: number) => formatInt(v),
    },
    {
      title: t('litellmops.bills.amount'),
      dataIndex: 'spend',
      render: (v: number) => formatUsd(v, 6),
    },
  ];

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Alert type="info" showIcon message={t('litellmops.bills.policy')} />
      {gatewayModels.isError ? (
        <Alert type="warning" showIcon message={t('litellmops.bills.modelOptionsUnavailable')} />
      ) : null}
      <Card
        title={t('litellmops.bills.title')}
        extra={
          <Space wrap>
            <Select
              allowClear
              showSearch
              placeholder={t('litellmops.gateway.model')}
              style={{ width: 220 }}
              loading={gatewayModels.isFetching}
              options={modelOptions}
              onChange={(model) => setParams((p) => ({ ...p, page: 1, model }))}
            />
            <Select
              allowClear
              placeholder={t('litellmops.common.status')}
              style={{ width: 120 }}
              options={[
                { value: 'success', label: t('litellmops.bills.success') },
                { value: 'failure', label: t('litellmops.bills.failure') },
              ]}
              onChange={(status) => setParams((p) => ({ ...p, page: 1, status }))}
            />
            <DatePicker.RangePicker
              value={range}
              onChange={(value) => {
                const next = value ?? [null, null];
                setRange([next[0] ?? null, next[1] ?? null]);
                setParams((p) => ({
                  ...p,
                  page: 1,
                  since: next[0] ? dayjs(next[0]).startOf('day').toISOString() : undefined,
                  until: next[1]
                    ? dayjs(next[1]).add(1, 'day').startOf('day').toISOString()
                    : undefined,
                }));
              }}
            />
            <Input.Search
              placeholder={t('litellmops.bills.keyPrefix')}
              allowClear
              style={{ width: 180 }}
              onSearch={(key_hash_prefix) =>
                setParams((p) => ({ ...p, page: 1, key_hash_prefix: key_hash_prefix || undefined }))
              }
            />
          </Space>
        }
      >
        <OpsQueryState
          data={data}
          error={bills.error}
          isError={bills.isError}
          isPending={bills.isPending}
          onRetry={() => void bills.refetch()}
        >
          <Row gutter={12} style={{ marginBottom: 16 }}>
            <Col span={8}>
              <Card size="small">
                <Statistic
                  title={t('litellmops.bills.totalSpend')}
                  value={data?.spend_sum ?? 0}
                  precision={4}
                  prefix="$"
                />
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small">
                <Statistic title={t('litellmops.bills.totalTokens')} value={data?.token_sum ?? 0} />
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small">
                <Statistic title={t('litellmops.bills.records')} value={data?.total ?? 0} />
              </Card>
            </Col>
          </Row>
          <Table<SpendLog>
            rowKey="request_id"
            loading={bills.isFetching}
            columns={columns}
            dataSource={data?.items ?? []}
            pagination={{
              current: params.page,
              pageSize: params.page_size,
              total: data?.total,
              showSizeChanger: false,
              onChange: (page) => setParams((p) => ({ ...p, page })),
            }}
          />
        </OpsQueryState>
      </Card>
    </Space>
  );
}
