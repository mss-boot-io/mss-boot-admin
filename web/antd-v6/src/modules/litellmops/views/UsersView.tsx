import { getRequestErrorMessage } from '@mss-admin-core/shared/api/errors';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl, useModel } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Switch,
  Table,
  type TableColumnsType,
  Tag,
} from 'antd';
import { useMemo, useState } from 'react';
import {
  canReconcileRecharge,
  createBusinessReference,
  isManagementPendingResponse,
  rechargeBudgetMicroRange,
} from '../business-reference';
import type {
  ManagedKey,
  ManagedUser,
  ManagedUserInput,
  ManagedUserPatch,
  RechargeRecord,
  RechargeRequest,
  UserListParams,
} from '../contract';
import { formatDateTime, formatInteger, formatUsd, formatUsdMicro, modelsOf } from '../contract';
import { ManagementRecoveryPanel } from '../ManagementRecoveryPanel';
import { operationsAPI } from '../operations-api';
import {
  useGatewayModels,
  useManagedUser,
  useManagedUsers,
  useUserRecharges,
} from '../operations-query';
import { buildOpsAccess } from '../permissions';
import { OpsQueryState } from '../ui';

type T = (id: string, values?: Record<string, string | number>) => string;
type EditorMode = 'create' | 'invite' | 'edit';

interface UserFormValues {
  email: string;
  user_role: string;
  models: string[];
  max_budget?: number;
  budget_duration?: string;
  tpm_limit?: number;
  rpm_limit?: number;
  blocked?: boolean;
}

interface RechargeValues {
  amount: number;
  reason: string;
  raise_keys: boolean;
  business_reference: string;
}

function UserDrawer({
  id,
  canRecharge,
  onClose,
  t,
}: {
  id?: string;
  canRecharge: boolean;
  onClose: () => void;
  t: T;
}) {
  const { message, modal } = App.useApp();
  const client = useQueryClient();
  const detail = useManagedUser(id);
  const history = useUserRecharges(id);
  const reconcile = useMutation({
    mutationFn: operationsAPI.recharges.reconcile,
    onSuccess: (record) =>
      message.info(
        t('litellmops.users.rechargeAccepted', {
          status: t(`litellmops.recharge.status.${record.status}`),
        }),
      ),
    onError: (error) => message.error(getRequestErrorMessage(error)),
    onSettled: () => client.invalidateQueries({ queryKey: ['litellmops'] }),
  });
  return (
    <Drawer
      destroyOnHidden
      title={t('litellmops.users.detail')}
      width="min(920px, 100vw)"
      open={Boolean(id)}
      onClose={onClose}
    >
      <OpsQueryState
        data={detail.data}
        error={detail.error}
        isError={detail.isError}
        isPending={detail.isPending}
        onRetry={() => void detail.refetch()}
      >
        {detail.data ? (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
              <Descriptions.Item label={t('litellmops.users.email')}>
                {detail.data.user.email}
              </Descriptions.Item>
              <Descriptions.Item label={t('litellmops.users.role')}>
                {detail.data.user.user_role}
              </Descriptions.Item>
              <Descriptions.Item label={t('litellmops.users.budget')}>
                {formatUsd(detail.data.user.max_budget)}
              </Descriptions.Item>
              <Descriptions.Item label={t('litellmops.users.spend')}>
                {formatUsd(detail.data.user.spend, 4)}
              </Descriptions.Item>
              <Descriptions.Item label={t('litellmops.users.tpm')}>
                {formatInteger(detail.data.user.tpm_limit)}
              </Descriptions.Item>
              <Descriptions.Item label={t('litellmops.users.rpm')}>
                {formatInteger(detail.data.user.rpm_limit)}
              </Descriptions.Item>
              <Descriptions.Item label={t('litellmops.users.models')} span={2}>
                <Space wrap>
                  {modelsOf(detail.data.user.models).map((model) => (
                    <Tag key={model}>{model}</Tag>
                  ))}
                </Space>
              </Descriptions.Item>
            </Descriptions>
            <Card size="small" title={t('litellmops.users.keys')}>
              <Table<ManagedKey>
                rowKey="id"
                size="small"
                pagination={false}
                dataSource={detail.data.keys}
                scroll={{ x: 680 }}
                columns={[
                  { title: t('litellmops.keys.alias'), dataIndex: 'alias' },
                  {
                    title: t('litellmops.keys.prefix'),
                    dataIndex: 'key_hash_prefix',
                    render: (value) => <code>{value}…</code>,
                  },
                  {
                    title: t('litellmops.users.budget'),
                    dataIndex: 'max_budget',
                    render: (value) => formatUsd(value),
                  },
                  {
                    title: t('litellmops.users.spend'),
                    dataIndex: 'spend',
                    render: (value) => formatUsd(value, 4),
                  },
                  {
                    title: t('litellmops.users.tpm'),
                    dataIndex: 'tpm_limit',
                    render: formatInteger,
                  },
                ]}
              />
            </Card>
            <Card size="small" title={t('litellmops.users.rechargeHistory')}>
              <OpsQueryState
                data={history.data}
                error={history.error}
                isError={history.isError}
                isPending={history.isPending}
                onRetry={() => void history.refetch()}
              >
                <Table<RechargeRecord>
                  rowKey="id"
                  size="small"
                  pagination={false}
                  dataSource={history.data?.items ?? []}
                  locale={{ emptyText: <Empty description={t('litellmops.users.noRecharges')} /> }}
                  scroll={{ x: 1_180 }}
                  columns={[
                    {
                      title: t('litellmops.common.createdAt'),
                      dataIndex: 'created_at',
                      render: formatDateTime,
                    },
                    {
                      title: t('litellmops.users.businessReference'),
                      render: (_, row) => (
                        <code>{row.business_reference || row.idempotency_key || row.id}</code>
                      ),
                    },
                    {
                      title: t('litellmops.users.amount'),
                      dataIndex: 'amount_usd_micro',
                      render: formatUsdMicro,
                    },
                    {
                      title: t('litellmops.users.beforeAfter'),
                      render: (_, row) => {
                        const range = rechargeBudgetMicroRange(row);
                        return range
                          ? `${formatUsdMicro(range[0])} → ${formatUsdMicro(range[1])}`
                          : t('litellmops.users.budgetUnknown');
                      },
                    },
                    {
                      title: t('litellmops.users.raiseKeys'),
                      dataIndex: 'raise_keys',
                      render: (value) =>
                        t(value ? 'litellmops.common.yes' : 'litellmops.common.no'),
                    },
                    {
                      title: t('litellmops.common.status'),
                      dataIndex: 'status',
                      render: (value) => <Tag>{t(`litellmops.recharge.status.${value}`)}</Tag>,
                    },
                    {
                      title: t('litellmops.common.error'),
                      dataIndex: 'last_error_code',
                      render: (value) => value || '—',
                    },
                    {
                      title: t('litellmops.common.actions'),
                      fixed: 'right',
                      render: (_, row) =>
                        canRecharge && canReconcileRecharge(row) ? (
                          <Button
                            size="small"
                            loading={reconcile.isPending && reconcile.variables === row.id}
                            onClick={() =>
                              modal.confirm({
                                title: t('litellmops.users.reconcile'),
                                content: t('litellmops.users.reconcileWarning'),
                                onOk: () => reconcile.mutateAsync(row.id),
                              })
                            }
                          >
                            {t('litellmops.users.reconcile')}
                          </Button>
                        ) : null,
                    },
                  ]}
                />
              </OpsQueryState>
            </Card>
          </Space>
        ) : null}
      </OpsQueryState>
    </Drawer>
  );
}

export default function UsersView() {
  const intl = useIntl();
  const t: T = (id, values) => intl.formatMessage({ id }, values);
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const access = useMemo(
    () => buildOpsAccess(initialState?.currentUser),
    [initialState?.currentUser],
  );
  const { message, modal } = App.useApp();
  const client = useQueryClient();
  const [params, setParams] = useState<UserListParams>({ page: 1, page_size: 20 });
  const [detailId, setDetailId] = useState<string>();
  const [editor, setEditor] = useState<{ mode: EditorMode; user?: ManagedUser }>();
  const [rechargeTarget, setRechargeTarget] = useState<{ user: ManagedUser; reference: string }>();
  const [userForm] = Form.useForm<UserFormValues>();
  const [rechargeForm] = Form.useForm<RechargeValues>();
  const users = useManagedUsers(params);
  const gatewayModels = useGatewayModels();

  const modelOptions = useMemo(() => {
    const names = new Set((gatewayModels.data?.items ?? []).map((model) => model.name));
    for (const model of modelsOf(editor?.user?.models)) names.add(model);
    return [...names].sort().map((value) => ({ value, label: value }));
  }, [editor?.user?.models, gatewayModels.data?.items]);

  const refresh = () => client.invalidateQueries({ queryKey: ['litellmops'] });
  const save = useMutation({
    mutationFn: ({
      mode,
      user,
      values,
    }: {
      mode: EditorMode;
      user?: ManagedUser;
      values: UserFormValues;
    }) => {
      const payload: ManagedUserInput = {
        email: values.email.trim(),
        user_role: values.user_role,
        models: values.models ?? [],
        max_budget: values.max_budget,
        budget_duration: values.budget_duration?.trim() || undefined,
        tpm_limit: values.tpm_limit,
        rpm_limit: values.rpm_limit,
      };
      if (mode === 'create') return operationsAPI.users.create(payload);
      if (mode === 'invite') return operationsAPI.users.invite(payload);
      const patch: ManagedUserPatch = { ...payload, blocked: values.blocked };
      return operationsAPI.users.update(user?.id as string, patch);
    },
    onSuccess: (result) => {
      message[isManagementPendingResponse(result) ? 'warning' : 'success'](
        t(
          isManagementPendingResponse(result)
            ? 'litellmops.management.pending'
            : 'litellmops.common.saved',
        ),
      );
      setEditor(undefined);
      userForm.resetFields();
    },
    onError: (error) => message.error(getRequestErrorMessage(error)),
    onSettled: refresh,
  });
  const toggle = useMutation({
    mutationFn: (user: ManagedUser) =>
      user.blocked ? operationsAPI.users.unblock(user.id) : operationsAPI.users.block(user.id),
    onSuccess: (result) =>
      message[isManagementPendingResponse(result) ? 'warning' : 'success'](
        t(
          isManagementPendingResponse(result)
            ? 'litellmops.management.pending'
            : 'litellmops.users.statusUpdated',
        ),
      ),
    onError: (error) => message.error(getRequestErrorMessage(error)),
    onSettled: refresh,
  });
  const remove = useMutation({
    mutationFn: operationsAPI.users.remove,
    onSuccess: (result) =>
      message[isManagementPendingResponse(result) ? 'warning' : 'success'](
        t(
          isManagementPendingResponse(result)
            ? 'litellmops.management.pending'
            : 'litellmops.common.deleted',
        ),
      ),
    onError: (error) => message.error(getRequestErrorMessage(error)),
    onSettled: refresh,
  });
  const sync = useMutation({
    mutationFn: operationsAPI.users.sync,
    onSuccess: async (report) => {
      await refresh();
      message.success(t('litellmops.users.synced', report));
    },
    onError: (error) => message.error(getRequestErrorMessage(error)),
  });
  const recharge = useMutation({
    mutationFn: ({ id, values }: { id: string; values: RechargeValues }) => {
      const reference = values.business_reference.trim();
      const payload: RechargeRequest = {
        amount: values.amount,
        reason: values.reason.trim(),
        raise_keys: values.raise_keys,
        business_reference: reference,
        idempotency_key: reference,
      };
      return operationsAPI.users.recharge(id, payload);
    },
    onSuccess: (record) => {
      message.success(
        t('litellmops.users.rechargeAccepted', {
          status: t(`litellmops.recharge.status.${record.status}`),
        }),
      );
      setRechargeTarget(undefined);
      rechargeForm.resetFields();
    },
    onError: (error) => message.error(getRequestErrorMessage(error)),
    onSettled: refresh,
  });

  if (!access.canReadUsers) return <PageForbidden />;

  const openEditor = (mode: EditorMode, user?: ManagedUser) => {
    setEditor({ mode, user });
    userForm.setFieldsValue(
      user
        ? {
            email: user.email,
            user_role: user.user_role,
            models: modelsOf(user.models),
            max_budget: user.max_budget ?? undefined,
            budget_duration: user.budget_duration ?? undefined,
            tpm_limit: user.tpm_limit ?? undefined,
            rpm_limit: user.rpm_limit ?? undefined,
            blocked: user.blocked,
          }
        : {
            email: '',
            user_role: 'internal_user',
            models: [],
            max_budget: 5,
            tpm_limit: 600_000,
          },
    );
  };

  const openRecharge = (user: ManagedUser) => {
    const reference = createBusinessReference('manual-credit');
    setRechargeTarget({ user, reference });
    rechargeForm.setFieldsValue({
      amount: 1,
      reason: '',
      raise_keys: true,
      business_reference: reference,
    });
  };

  const columns: TableColumnsType<ManagedUser> = [
    {
      title: t('litellmops.users.email'),
      dataIndex: 'email',
      fixed: 'left',
      width: 220,
      render: (value) => value || '—',
    },
    {
      title: t('litellmops.users.role'),
      dataIndex: 'user_role',
      width: 160,
      render: (value) => <Tag>{value}</Tag>,
    },
    {
      title: t('litellmops.users.models'),
      dataIndex: 'models',
      width: 110,
      render: (value) => t('litellmops.users.modelCount', { count: modelsOf(value).length }),
    },
    {
      title: t('litellmops.users.budget'),
      dataIndex: 'max_budget',
      width: 110,
      render: (value) => formatUsd(value),
    },
    {
      title: t('litellmops.users.spend'),
      dataIndex: 'spend',
      width: 110,
      render: (value) => formatUsd(value, 4),
    },
    { title: t('litellmops.users.tpm'), dataIndex: 'tpm_limit', width: 110, render: formatInteger },
    { title: t('litellmops.users.rpm'), dataIndex: 'rpm_limit', width: 100, render: formatInteger },
    {
      title: t('litellmops.common.status'),
      dataIndex: 'blocked',
      width: 100,
      render: (blocked) => (
        <Tag color={blocked ? 'error' : 'success'}>
          {t(blocked ? 'litellmops.common.disabled' : 'litellmops.common.enabled')}
        </Tag>
      ),
    },
    {
      title: t('litellmops.users.syncedAt'),
      dataIndex: 'synced_at',
      width: 180,
      render: formatDateTime,
    },
    {
      title: t('litellmops.common.actions'),
      key: 'actions',
      fixed: 'right',
      width: 300,
      render: (_, user) => (
        <Space size={2} wrap>
          {access.canReadUserDetails ? (
            <Button type="link" size="small" onClick={() => setDetailId(user.id)}>
              {t('litellmops.common.detail')}
            </Button>
          ) : null}
          {access.canRecharge ? (
            <Button size="small" onClick={() => openRecharge(user)}>
              {t('litellmops.users.recharge')}
            </Button>
          ) : null}
          {access.canWriteUsers ? (
            <>
              <Button type="link" size="small" onClick={() => openEditor('edit', user)}>
                {t('litellmops.common.edit')}
              </Button>
              <Button
                size="small"
                danger={!user.blocked}
                onClick={() =>
                  modal.confirm({
                    title: t(
                      user.blocked ? 'litellmops.common.unblock' : 'litellmops.common.block',
                    ),
                    content: t('litellmops.users.blockWarning'),
                    onOk: () => toggle.mutateAsync(user),
                  })
                }
              >
                {t(user.blocked ? 'litellmops.common.unblock' : 'litellmops.common.block')}
              </Button>
              <Button
                type="link"
                danger
                size="small"
                onClick={() =>
                  modal.confirm({
                    title: t('litellmops.users.deleteTitle'),
                    content: t('litellmops.users.deleteWarning'),
                    okButtonProps: { danger: true },
                    onOk: () => remove.mutateAsync(user.id),
                  })
                }
              >
                {t('litellmops.common.delete')}
              </Button>
            </>
          ) : null}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title={t('litellmops.users.title')} content={t('litellmops.users.description')}>
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Alert type="info" showIcon message={t('litellmops.users.policy')} />
        <Card
          extra={
            <Space wrap>
              <Input.Search
                allowClear
                placeholder={t('litellmops.users.search')}
                onSearch={(email) =>
                  setParams((current) => ({ ...current, page: 1, email: email || undefined }))
                }
              />
              <Select
                allowClear
                placeholder={t('litellmops.users.role')}
                style={{ minWidth: 170 }}
                options={[
                  { value: 'internal_user', label: 'internal_user' },
                  { value: 'internal_user_viewer', label: 'internal_user_viewer' },
                  { value: 'proxy_admin', label: 'proxy_admin' },
                ]}
                onChange={(user_role) =>
                  setParams((current) => ({ ...current, page: 1, user_role }))
                }
              />
              {access.canSync ? (
                <Button loading={sync.isPending} onClick={() => sync.mutate()}>
                  {t('litellmops.users.sync')}
                </Button>
              ) : null}
              {access.canInviteUsers ? (
                <Button onClick={() => openEditor('invite')}>{t('litellmops.users.invite')}</Button>
              ) : null}
              {access.canWriteUsers ? (
                <Button type="primary" onClick={() => openEditor('create')}>
                  {t('litellmops.users.create')}
                </Button>
              ) : null}
            </Space>
          }
        >
          <OpsQueryState
            data={users.data}
            error={users.error}
            isError={users.isError}
            isPending={users.isPending}
            onRetry={() => void users.refetch()}
          >
            <Table<ManagedUser>
              rowKey="id"
              loading={users.isFetching}
              columns={columns}
              dataSource={users.data?.items ?? []}
              locale={{ emptyText: <Empty description={t('litellmops.users.empty')} /> }}
              scroll={{ x: 1_500 }}
              pagination={{
                current: params.page,
                pageSize: params.page_size,
                total: users.data?.total,
                onChange: (page, pageSize) =>
                  setParams((current) => ({ ...current, page, page_size: pageSize })),
              }}
            />
          </OpsQueryState>
        </Card>
        <ManagementRecoveryPanel enabled={access.canResolveManagement} />
      </Space>

      <UserDrawer
        id={detailId}
        canRecharge={access.canRecharge}
        onClose={() => setDetailId(undefined)}
        t={t}
      />
      <Modal
        destroyOnHidden
        title={t(`litellmops.users.${editor?.mode ?? 'create'}`)}
        open={Boolean(editor)}
        confirmLoading={save.isPending}
        onCancel={() => setEditor(undefined)}
        onOk={() =>
          void userForm
            .validateFields()
            .then(
              (values) => editor && save.mutate({ mode: editor.mode, user: editor.user, values }),
            )
        }
      >
        <Alert showIcon type="info" message={t('litellmops.users.writeAudit')} />
        <Form form={userForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="email"
            label={t('litellmops.users.email')}
            rules={[{ required: true, type: 'email' }]}
          >
            <Input disabled={editor?.mode === 'edit'} autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="user_role"
            label={t('litellmops.users.role')}
            rules={[{ required: true }]}
          >
            <Select
              options={[
                { value: 'internal_user', label: 'internal_user' },
                { value: 'internal_user_viewer', label: 'internal_user_viewer' },
                { value: 'proxy_admin', label: 'proxy_admin' },
              ]}
            />
          </Form.Item>
          <Form.Item name="models" label={t('litellmops.users.models')}>
            <Select mode="multiple" showSearch optionFilterProp="label" options={modelOptions} />
          </Form.Item>
          <Form.Item name="max_budget" label={t('litellmops.users.budget')}>
            <InputNumber min={0} precision={2} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="budget_duration" label={t('litellmops.users.budgetDuration')}>
            <Input placeholder="7d" autoComplete="off" />
          </Form.Item>
          <Space wrap>
            <Form.Item name="tpm_limit" label={t('litellmops.users.tpm')}>
              <InputNumber min={0} precision={0} />
            </Form.Item>
            <Form.Item name="rpm_limit" label={t('litellmops.users.rpm')}>
              <InputNumber min={0} precision={0} />
            </Form.Item>
          </Space>
          {editor?.mode === 'edit' ? (
            <Form.Item name="blocked" label={t('litellmops.users.blocked')} valuePropName="checked">
              <Switch />
            </Form.Item>
          ) : null}
        </Form>
      </Modal>

      <Modal
        destroyOnHidden
        title={t('litellmops.users.rechargeTitle', { email: rechargeTarget?.user.email ?? '' })}
        open={Boolean(rechargeTarget)}
        confirmLoading={recharge.isPending}
        onCancel={() => setRechargeTarget(undefined)}
        onOk={() =>
          void rechargeForm
            .validateFields()
            .then(
              (values) => rechargeTarget && recharge.mutate({ id: rechargeTarget.user.id, values }),
            )
        }
      >
        <Alert type="warning" showIcon message={t('litellmops.users.rechargeWarning')} />
        <Form form={rechargeForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="business_reference"
            label={t('litellmops.users.businessReference')}
            rules={[{ required: true, min: 8 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="amount"
            label={t('litellmops.users.amount')}
            rules={[{ required: true }]}
          >
            <InputNumber min={0.01} max={10_000} precision={2} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="reason"
            label={t('litellmops.common.reason')}
            rules={[{ required: true, min: 4 }]}
          >
            <Input.TextArea rows={2} maxLength={500} />
          </Form.Item>
          <Form.Item name="raise_keys" valuePropName="checked">
            <Checkbox>{t('litellmops.users.raiseKeys')}</Checkbox>
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  );
}
