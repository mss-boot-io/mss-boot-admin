import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useModel } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Drawer,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
} from 'antd';
import { useMemo, useState } from 'react';
import { litellmopsAPI } from '@/modules/litellmops/api';
import { buildOpsAccess } from '@/modules/litellmops/permissions';
import { useOrgDetail, useOrgList } from '@/modules/litellmops/query';
import { formatUsd, type RemoteOrg, type RemoteTeam } from '@/modules/litellmops/types';

const MODEL_OPTIONS = [
  'grok-4.6',
  'gpt-5.6-luna',
  'grok-4.5',
  'grok-4.3',
  'claude-sonnet-4-6',
  'claude-opus-4-6',
  'claude-haiku-4-5',
].map((value) => ({ value, label: value }));

const MEMBER_ROLES = [
  { value: 'org_admin', label: 'org_admin' },
  { value: 'internal_user', label: 'internal_user' },
  { value: 'internal_user_viewer', label: 'internal_user_viewer' },
];

export default function LitellmOpsOrgsPage() {
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const access = useMemo(
    () => buildOpsAccess(initialState?.currentUser),
    [initialState?.currentUser],
  );
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const { data, isFetching, refetch } = useOrgList();
  const [detailId, setDetailId] = useState<string>();
  const [createOpen, setCreateOpen] = useState(false);
  const [editingOrg, setEditingOrg] = useState<RemoteOrg>();
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['litellmops'] });

  const createMutation = useMutation({
    mutationFn: litellmopsAPI.createOrg,
    onSuccess: () => {
      message.success('组织已创建');
      setCreateOpen(false);
      createForm.resetFields();
      void invalidate();
    },
    onError: (error) => message.error(`创建失败：${String(error)}`),
  });

  const deleteMutation = useMutation({
    mutationFn: litellmopsAPI.deleteOrg,
    onSuccess: () => {
      message.success('组织已删除');
      void invalidate();
    },
    onError: (error) => message.error(`删除失败：${String(error)}`),
  });

  const updateMutation = useMutation({
    mutationFn: ({
      id,
      body,
    }: {
      id: string;
      body: { organization_alias: string; max_budget?: number; models?: string[] };
    }) => litellmopsAPI.updateOrg(id, body),
    onSuccess: () => {
      message.success('组织已更新');
      setEditingOrg(undefined);
      editForm.resetFields();
      void invalidate();
    },
    onError: (error) => message.error(`更新失败：${String(error)}`),
  });

  if (!access.canReadOrganizations) return <PageForbidden />;

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Alert
        type="info"
        showIcon
        message="组织是 LiteLLM 的顶层账本。团队挂在组织下。写操作走 LiteLLM Admin API，不直改数据库。"
      />
      <Alert
        type="warning"
        showIcon
        message="组织与团队充值暂时安全关闭；在独立幂等流水和未决结果保护完成前不开放入口。"
      />
      <Card
        title="组织与团队"
        extra={
          <Space>
            <Button onClick={() => void refetch()}>刷新</Button>
            {access.canWriteOrganizations ? (
              <Button type="primary" onClick={() => setCreateOpen(true)}>
                新建组织
              </Button>
            ) : null}
          </Space>
        }
      >
        <Table<RemoteOrg>
          rowKey="organization_id"
          loading={isFetching}
          dataSource={data?.items}
          columns={[
            { title: '名称', dataIndex: 'organization_alias' },
            {
              title: 'ID',
              dataIndex: 'organization_id',
              render: (v: string) => <code>{v}</code>,
            },
            {
              title: '预算',
              dataIndex: 'max_budget',
              render: (v: number | null) => formatUsd(v, 2),
            },
            { title: '已消费', dataIndex: 'spend', render: (v: number) => formatUsd(v) },
            {
              title: '成员',
              render: (_: unknown, record) => record.members?.length ?? 0,
            },
            {
              title: '团队',
              render: (_: unknown, record) => record.teams?.length ?? 0,
            },
            {
              title: '操作',
              render: (_: unknown, record) => (
                <Space>
                  {access.canReadOrganizationDetails ? (
                    <Button
                      type="link"
                      size="small"
                      onClick={() => setDetailId(record.organization_id)}
                    >
                      详情
                    </Button>
                  ) : null}
                  {access.canWriteOrganizations ? (
                    <Button
                      type="link"
                      size="small"
                      onClick={() => {
                        setEditingOrg(record);
                        editForm.setFieldsValue({
                          organization_alias: record.organization_alias,
                          max_budget: record.max_budget,
                          models: record.models,
                        });
                      }}
                    >
                      编辑
                    </Button>
                  ) : null}
                  {access.canWriteOrganizations ? (
                    <Popconfirm
                      title="删除该组织？"
                      onConfirm={() => deleteMutation.mutate(record.organization_id)}
                    >
                      <Button type="link" size="small" danger>
                        删除
                      </Button>
                    </Popconfirm>
                  ) : null}
                </Space>
              ),
            },
          ]}
          pagination={false}
        />
      </Card>
      <OrgDetailDrawer
        canWrite={access.canWriteOrganizations}
        id={detailId}
        onClose={() => setDetailId(undefined)}
      />
      <Modal
        title="新建组织"
        open={createOpen}
        confirmLoading={createMutation.isPending}
        onCancel={() => setCreateOpen(false)}
        onOk={() => {
          void createForm.validateFields().then((values) => {
            createMutation.mutate({
              organization_alias: values.organization_alias,
              max_budget: values.max_budget,
              models: values.models,
            });
          });
        }}
      >
        <Form form={createForm} layout="vertical" initialValues={{ max_budget: 100 }}>
          <Form.Item
            name="organization_alias"
            label="名称"
            rules={[{ required: true, message: '请输入名称' }]}
          >
            <Input placeholder="例如 acme" />
          </Form.Item>
          <Form.Item name="max_budget" label="预算（USD）">
            <InputNumber min={0} max={10000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="models" label="授权模型">
            <Select mode="multiple" options={MODEL_OPTIONS} placeholder="留空则按 LiteLLM 默认" />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        title="编辑组织"
        open={Boolean(editingOrg)}
        confirmLoading={updateMutation.isPending}
        onCancel={() => setEditingOrg(undefined)}
        onOk={() =>
          void editForm
            .validateFields()
            .then(
              (body) =>
                editingOrg && updateMutation.mutate({ id: editingOrg.organization_id, body }),
            )
        }
      >
        <Form form={editForm} layout="vertical">
          <Form.Item name="organization_alias" label="名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="max_budget" label="预算（USD）">
            <InputNumber min={0} max={10000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="models" label="授权模型">
            <Select mode="multiple" options={MODEL_OPTIONS} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  );
}

function OrgDetailDrawer({
  canWrite,
  id,
  onClose,
}: {
  canWrite: boolean;
  id?: string;
  onClose: () => void;
}) {
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const { data } = useOrgDetail(id);
  const [memberForm] = Form.useForm();
  const [teamForm] = Form.useForm();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['litellmops'] });

  const addMember = useMutation({
    mutationFn: (body: { user_email: string; role: string }) =>
      litellmopsAPI.addOrgMember(id as string, body),
    onSuccess: () => {
      message.success('成员已加入');
      memberForm.resetFields();
      void invalidate();
    },
    onError: (error) => message.error(`加人失败：${String(error)}`),
  });
  const removeMember = useMutation({
    mutationFn: (body: { user_id?: string; user_email?: string }) =>
      litellmopsAPI.removeOrgMember(id as string, body),
    onSuccess: () => {
      message.success('成员已移除');
      void invalidate();
    },
    onError: (error) => message.error(`移除失败：${String(error)}`),
  });
  const createTeam = useMutation({
    mutationFn: (body: { team_alias: string; max_budget?: number; models?: string[] }) =>
      litellmopsAPI.createTeam(id as string, body),
    onSuccess: () => {
      message.success('团队已创建');
      teamForm.resetFields();
      void invalidate();
    },
    onError: (error) => message.error(`建团队失败：${String(error)}`),
  });
  const deleteTeam = useMutation({
    mutationFn: litellmopsAPI.deleteTeam,
    onSuccess: () => {
      message.success('团队已删除');
      void invalidate();
    },
    onError: (error) => message.error(`删除失败：${String(error)}`),
  });
  return (
    <Drawer
      title={data?.organization_alias || '组织详情'}
      width={860}
      open={Boolean(id)}
      onClose={onClose}
    >
      {data && (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Descriptions size="small" bordered column={2}>
            <Descriptions.Item label="ID">
              <code>{data.organization_id}</code>
            </Descriptions.Item>
            <Descriptions.Item label="预算">{formatUsd(data.max_budget, 2)}</Descriptions.Item>
            <Descriptions.Item label="已消费">{formatUsd(data.spend)}</Descriptions.Item>
            <Descriptions.Item label="周期">{data.budget_duration || '—'}</Descriptions.Item>
            <Descriptions.Item label="模型" span={2}>
              {(data.models || []).length === 0
                ? '—'
                : data.models.map((model) => <Tag key={model}>{model}</Tag>)}
            </Descriptions.Item>
          </Descriptions>

          <Card size="small" title="成员">
            {canWrite ? (
              <Form
                form={memberForm}
                layout="inline"
                style={{ marginBottom: 12 }}
                initialValues={{ role: 'internal_user' }}
                onFinish={(values) => addMember.mutate(values)}
              >
                <Form.Item name="user_email" rules={[{ required: true, message: '邮箱' }]}>
                  <Input placeholder="用户邮箱" style={{ width: 220 }} />
                </Form.Item>
                <Form.Item name="role">
                  <Select options={MEMBER_ROLES} style={{ width: 180 }} />
                </Form.Item>
                <Form.Item>
                  <Button type="primary" htmlType="submit" loading={addMember.isPending}>
                    加入
                  </Button>
                </Form.Item>
              </Form>
            ) : null}
            <Table
              rowKey={(row) => row.user_id || row.user_email}
              size="small"
              pagination={false}
              dataSource={data.members}
              columns={[
                { title: '邮箱', dataIndex: 'user_email' },
                { title: '角色', dataIndex: 'role', render: (v: string) => <Tag>{v || '—'}</Tag> },
                {
                  title: '操作',
                  render: (_: unknown, record) =>
                    canWrite ? (
                      <Button
                        type="link"
                        size="small"
                        danger
                        onClick={() =>
                          removeMember.mutate({
                            user_id: record.user_id,
                            user_email: record.user_email,
                          })
                        }
                      >
                        移除
                      </Button>
                    ) : null,
                },
              ]}
            />
          </Card>

          <Card size="small" title="团队">
            {canWrite ? (
              <Form
                form={teamForm}
                layout="inline"
                style={{ marginBottom: 12 }}
                onFinish={(values) => createTeam.mutate(values)}
              >
                <Form.Item name="team_alias" rules={[{ required: true, message: '名称' }]}>
                  <Input placeholder="团队名称" />
                </Form.Item>
                <Form.Item name="max_budget">
                  <InputNumber min={0} placeholder="预算" />
                </Form.Item>
                <Form.Item>
                  <Button htmlType="submit" loading={createTeam.isPending}>
                    新建团队
                  </Button>
                </Form.Item>
              </Form>
            ) : null}
            <Table<RemoteTeam>
              rowKey="team_id"
              size="small"
              pagination={false}
              dataSource={data.teams}
              columns={[
                { title: '名称', dataIndex: 'team_alias' },
                {
                  title: '预算',
                  dataIndex: 'max_budget',
                  render: (v: number | null) => formatUsd(v, 2),
                },
                { title: '已消费', dataIndex: 'spend', render: (v: number) => formatUsd(v) },
                {
                  title: '操作',
                  render: (_: unknown, record) =>
                    canWrite ? (
                      <Space>
                        <Popconfirm
                          title="删除团队？"
                          onConfirm={() => deleteTeam.mutate(record.team_id)}
                        >
                          <Button type="link" size="small" danger>
                            删除
                          </Button>
                        </Popconfirm>
                      </Space>
                    ) : null,
                },
              ]}
            />
          </Card>
        </Space>
      )}
    </Drawer>
  );
}
