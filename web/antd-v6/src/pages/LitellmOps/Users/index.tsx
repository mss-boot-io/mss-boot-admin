import {
  Button,
  Card,
  Descriptions,
  Drawer,
  Input,
  message,
  Popover,
  Select,
  Space,
  Table,
  Tag,
} from 'antd';
import { useState } from 'react';
import { useSyncMutation, useUserDetail, useUserPage } from '@/modules/litellmops/query';
import {
  formatDateTime,
  formatInt,
  formatUsd,
  type KeySnapshot,
  parseModels,
  type UserPageParams,
  type UserSnapshot,
} from '@/modules/litellmops/types';

function ModelsCell({ models }: { models: string }) {
  const parsed = parseModels(models);
  if (parsed.length === 0) return <span>—</span>;
  return (
    <Popover
      content={
        <Space wrap size={4}>
          {parsed.map((model) => (
            <Tag key={model}>{model}</Tag>
          ))}
        </Space>
      }
    >
      <Tag color="blue">{parsed.length} 个模型</Tag>
    </Popover>
  );
}

function UserDetailDrawer({ id, onClose }: { id?: string; onClose: () => void }) {
  const { data } = useUserDetail(id);
  const keyColumns = [
    { title: '别名', dataIndex: 'alias', render: (v: string | null) => v ?? '—' },
    {
      title: '哈希前缀',
      dataIndex: 'key_hash_prefix',
      render: (v: string) => <code>{v}…</code>,
    },
    { title: '预算', dataIndex: 'max_budget', render: (v: number | null) => formatUsd(v, 2) },
    { title: '已用', dataIndex: 'spend', render: (v: number) => formatUsd(v) },
    { title: 'TPM', dataIndex: 'tpm_limit', render: (v: number | null) => formatInt(v) },
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
  ];
  return (
    <Drawer title="用户详情" width={860} open={Boolean(id)} onClose={onClose}>
      {data?.user && (
        <>
          <Descriptions column={2} size="small" bordered>
            <Descriptions.Item label="邮箱">{data.user.email || '—'}</Descriptions.Item>
            <Descriptions.Item label="角色">{data.user.user_role || '—'}</Descriptions.Item>
            <Descriptions.Item label="预算上限">
              {formatUsd(data.user.max_budget, 2)}
            </Descriptions.Item>
            <Descriptions.Item label="已消费">{formatUsd(data.user.spend)}</Descriptions.Item>
            <Descriptions.Item label="预算周期">
              {data.user.budget_duration ?? '一次性'}
            </Descriptions.Item>
            <Descriptions.Item label="重置时间">
              {formatDateTime(data.user.budget_reset_at)}
            </Descriptions.Item>
            <Descriptions.Item label="授权模型" span={2}>
              <ModelsCell models={data.user.models} />
            </Descriptions.Item>
            <Descriptions.Item label="同步时间" span={2}>
              {formatDateTime(data.user.synced_at)}
            </Descriptions.Item>
          </Descriptions>
          <h3 style={{ marginTop: 16 }}>Key 列表</h3>
          <Table<KeySnapshot>
            rowKey="id"
            size="small"
            columns={keyColumns}
            dataSource={data.keys}
            pagination={false}
          />
        </>
      )}
    </Drawer>
  );
}

export default function LitellmOpsUsersPage() {
  const [params, setParams] = useState<UserPageParams>({ page: 1, page_size: 20 });
  const [detailId, setDetailId] = useState<string>();
  const { data, isFetching } = useUserPage(params);
  const syncMutation = useSyncMutation();

  const runSync = () => {
    syncMutation.mutate(undefined, {
      onSuccess: (report) =>
        message.success(
          `同步完成：用户 ${report.users}、Key ${report.keys}、下线 ${report.users_retired + report.keys_retired}`,
        ),
      onError: (error) => message.error(`同步失败：${String(error)}`),
    });
  };

  const columns = [
    { title: '邮箱', dataIndex: 'email', render: (v: string) => v || '—' },
    {
      title: '角色',
      dataIndex: 'user_role',
      render: (v: string) => (v ? <Tag>{v}</Tag> : '—'),
    },
    { title: '授权模型', dataIndex: 'models', render: (v: string) => <ModelsCell models={v} /> },
    { title: '预算上限', dataIndex: 'max_budget', render: (v: number | null) => formatUsd(v, 2) },
    {
      title: '周期',
      dataIndex: 'budget_duration',
      render: (v: string | null) => (v ? <Tag>{v}</Tag> : <Tag color="purple">一次性</Tag>),
    },
    { title: '已消费', dataIndex: 'spend', render: (v: number) => formatUsd(v) },
    {
      title: '重置时间',
      dataIndex: 'budget_reset_at',
      render: (v: string | null) => formatDateTime(v),
    },
    { title: '同步时间', dataIndex: 'synced_at', render: (v: string) => formatDateTime(v) },
    {
      title: '操作',
      render: (_: unknown, record: UserSnapshot) => (
        <Button type="link" size="small" onClick={() => setDetailId(record.id)}>
          详情
        </Button>
      ),
    },
  ];

  return (
    <Card
      title="用户与额度"
      extra={
        <Space>
          <Input.Search
            placeholder="邮箱过滤"
            allowClear
            style={{ width: 220 }}
            onSearch={(email) => setParams((p) => ({ ...p, page: 1, email: email || undefined }))}
          />
          <Select
            allowClear
            placeholder="角色"
            style={{ width: 160 }}
            options={[
              { value: 'internal_user', label: 'internal_user' },
              { value: 'proxy_admin', label: 'proxy_admin' },
            ]}
            onChange={(user_role) => setParams((p) => ({ ...p, page: 1, user_role }))}
          />
          <Button type="primary" loading={syncMutation.isPending} onClick={runSync}>
            手动同步
          </Button>
        </Space>
      }
    >
      <Table<UserSnapshot>
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
      <UserDetailDrawer id={detailId} onClose={() => setDetailId(undefined)} />
    </Card>
  );
}
