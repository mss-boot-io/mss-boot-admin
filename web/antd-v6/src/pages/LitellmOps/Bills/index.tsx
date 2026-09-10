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
import { useBillsPage } from '@/modules/litellmops/query';
import {
  type BillsParams,
  formatDateTime,
  formatInt,
  formatUsd,
  type SpendLog,
} from '@/modules/litellmops/types';

const MODEL_OPTIONS = [
  'grok-4.6',
  'grok-4.5',
  'grok-4.3',
  'grok-4.20-0309-reasoning',
  'grok-4.20-0309-non-reasoning',
  'grok-4.20-multi-agent-0309',
  'grok-build-0.1',
  'gpt-5.6-luna',
  'claude-sonnet-4-6',
  'claude-opus-4-6',
  'claude-haiku-4-5',
].map((value) => ({ value, label: value }));

export default function LitellmOpsBillsPage() {
  const [params, setParams] = useState<BillsParams>({ page: 1, page_size: 20 });
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null]>([null, null]);
  const { data, isFetching } = useBillsPage(params);

  const columns = [
    { title: '时间', dataIndex: 'start_time', render: (v: string) => formatDateTime(v) },
    { title: '模型', dataIndex: 'model', render: (v: string) => <code>{v || '—'}</code> },
    {
      title: '状态',
      dataIndex: 'status',
      render: (v: string) =>
        v === 'success' ? <Tag color="green">成功</Tag> : <Tag color="red">{v || '失败'}</Tag>,
    },
    { title: '输入 Token', dataIndex: 'prompt_tokens', render: (v: number) => formatInt(v) },
    { title: '输出 Token', dataIndex: 'completion_tokens', render: (v: number) => formatInt(v) },
    { title: '总 Token', dataIndex: 'total_tokens', render: (v: number) => formatInt(v) },
    { title: '金额', dataIndex: 'spend', render: (v: number) => formatUsd(v, 6) },
  ];

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Alert
        type="info"
        showIcon
        message="计费口径：gpt-5.6-luna 200K–272K 输入按高费率计；缓存写 1.25x 未配置；流式修复前历史记录存在估算偏差（未回填）。"
      />
      <Card
        title="账单"
        extra={
          <Space wrap>
            <Select
              allowClear
              placeholder="模型"
              style={{ width: 220 }}
              options={MODEL_OPTIONS}
              onChange={(model) => setParams((p) => ({ ...p, page: 1, model }))}
            />
            <Select
              allowClear
              placeholder="状态"
              style={{ width: 120 }}
              options={[
                { value: 'success', label: '成功' },
                { value: 'failure', label: '失败' },
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
              placeholder="Key 哈希前缀"
              allowClear
              style={{ width: 180 }}
              onSearch={(key_hash_prefix) =>
                setParams((p) => ({ ...p, page: 1, key_hash_prefix: key_hash_prefix || undefined }))
              }
            />
          </Space>
        }
      >
        <Row gutter={12} style={{ marginBottom: 16 }}>
          <Col span={8}>
            <Card size="small">
              <Statistic title="总消费" value={data?.spend_sum ?? 0} precision={4} prefix="$" />
            </Card>
          </Col>
          <Col span={8}>
            <Card size="small">
              <Statistic title="总 Token" value={data?.token_sum ?? 0} />
            </Card>
          </Col>
          <Col span={8}>
            <Card size="small">
              <Statistic title="记录数" value={data?.total ?? 0} />
            </Card>
          </Col>
        </Row>
        <Table<SpendLog>
          rowKey="request_id"
          loading={isFetching}
          columns={columns}
          dataSource={data?.items}
          pagination={{
            current: params.page,
            pageSize: params.page_size,
            total: data?.total,
            showSizeChanger: false,
            onChange: (page) => setParams((p) => ({ ...p, page })),
          }}
        />
      </Card>
    </Space>
  );
}
