import { Card, Input, Select, Space, Table, Tag } from 'antd';
import { useState } from 'react';
import { useKeyPage } from '@/modules/litellmops/query';
import {
  formatDateTime,
  formatInt,
  formatUsd,
  type KeyPageParams,
  type KeySnapshot,
} from '@/modules/litellmops/types';

export default function LitellmOpsKeysPage() {
  const [params, setParams] = useState<KeyPageParams>({ page: 1, page_size: 20 });
  const { data, isFetching } = useKeyPage(params);

  const columns = [
    {
      title: '别名',
      dataIndex: 'alias',
      render: (v: string | null) => (v ? <Tag color="blue">{v}</Tag> : '—'),
    },
    {
      title: '哈希前缀',
      dataIndex: 'key_hash_prefix',
      render: (v: string) => <code>{v}…</code>,
    },
    { title: '归属邮箱', dataIndex: 'user_email', render: (v: string) => v || '—' },
    { title: '预算', dataIndex: 'max_budget', render: (v: number | null) => formatUsd(v, 2) },
    { title: '已用', dataIndex: 'spend', render: (v: number) => formatUsd(v) },
    { title: 'TPM', dataIndex: 'tpm_limit', render: (v: number | null) => formatInt(v) },
    { title: 'RPM', dataIndex: 'rpm_limit', render: (v: number | null) => formatInt(v) },
    {
      title: '过期',
      dataIndex: 'expires',
      render: (v: string | null) => formatDateTime(v),
    },
    {
      title: '类型',
      dataIndex: 'is_session_key',
      render: (v: boolean) =>
        v ? <Tag color="orange">UI 会话</Tag> : <Tag color="green">真实 Key</Tag>,
    },
    { title: '同步时间', dataIndex: 'synced_at', render: (v: string) => formatDateTime(v) },
  ];

  return (
    <Card
      title="Key 快照"
      extra={
        <Space>
          <Input.Search
            placeholder="别名过滤"
            allowClear
            style={{ width: 160 }}
            onSearch={(alias) => setParams((p) => ({ ...p, page: 1, alias: alias || undefined }))}
          />
          <Input.Search
            placeholder="归属邮箱过滤"
            allowClear
            style={{ width: 200 }}
            onSearch={(user_email) =>
              setParams((p) => ({ ...p, page: 1, user_email: user_email || undefined }))
            }
          />
          <Select
            allowClear
            placeholder="类型"
            style={{ width: 140 }}
            options={[
              { value: 'false', label: '仅真实 Key' },
              { value: 'true', label: '仅 UI 会话' },
            ]}
            onChange={(is_session_key) => setParams((p) => ({ ...p, page: 1, is_session_key }))}
          />
        </Space>
      }
    >
      <Table<KeySnapshot>
        rowKey="id"
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
  );
}
