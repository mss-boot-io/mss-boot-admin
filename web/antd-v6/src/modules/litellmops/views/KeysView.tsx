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
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  type TableColumnsType,
  Tag,
  Typography,
} from 'antd';
import { useMemo, useState } from 'react';
import { isManagementPendingResponse, managementMutationOutcome } from '../business-reference';
import type { KeyListParams, ManagedKey, ManagedKeyInput, ManagedKeyPatch } from '../contract';
import { formatDateTime, formatInteger, formatUsd, modelsOf } from '../contract';
import { ManagementRecoveryPanel } from '../ManagementRecoveryPanel';
import { operationsAPI } from '../operations-api';
import { useGatewayModels, useManagedKeys } from '../operations-query';
import { buildOpsAccess } from '../permissions';
import { OpsQueryState } from '../ui';

type T = (id: string, values?: Record<string, string | number>) => string;
type KeyAction = 'block' | 'unblock' | 'rotate' | 'reset-spend' | 'remove';

interface KeyFormValues {
  alias: string;
  user_email?: string;
  models: string[];
  max_budget?: number;
  tpm_limit?: number;
  rpm_limit?: number;
  max_parallel_requests?: number;
  expires?: string;
}

export default function KeysView() {
  const intl = useIntl();
  const t: T = (id, values) => intl.formatMessage({ id }, values);
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const access = useMemo(
    () => buildOpsAccess(initialState?.currentUser),
    [initialState?.currentUser],
  );
  const { message, modal } = App.useApp();
  const client = useQueryClient();
  const [params, setParams] = useState<KeyListParams>({
    page: 1,
    page_size: 20,
    is_session_key: 'false',
  });
  const [editor, setEditor] = useState<{ key?: ManagedKey }>();
  const [oneTimeKey, setOneTimeKey] = useState<string>();
  const [form] = Form.useForm<KeyFormValues>();
  const keys = useManagedKeys(params);
  const gatewayModels = useGatewayModels();

  const modelOptions = useMemo(() => {
    const names = new Set((gatewayModels.data?.items ?? []).map((model) => model.name));
    for (const model of modelsOf(editor?.key?.models)) names.add(model);
    return [...names].sort().map((value) => ({ value, label: value }));
  }, [editor?.key?.models, gatewayModels.data?.items]);

  const refresh = () => client.invalidateQueries({ queryKey: ['litellmops'] });
  const revealSecret = (secret: string | undefined, clearMutation: () => void) => {
    clearMutation();
    setEditor(undefined);
    form.resetFields();
    if (secret) setOneTimeKey(secret);
    else message.warning(t('litellmops.keys.secretMissing'));
  };
  const save = useMutation({
    mutationFn: async ({ key, values }: { key?: ManagedKey; values: KeyFormValues }) => {
      const common = {
        alias: values.alias.trim(),
        models: values.models ?? [],
        max_budget: values.max_budget,
        tpm_limit: values.tpm_limit,
        rpm_limit: values.rpm_limit,
        max_parallel_requests: values.max_parallel_requests,
        expires: values.expires?.trim() || undefined,
      };
      if (key) return await operationsAPI.keys.update(key.id, common satisfies ManagedKeyPatch);
      return await operationsAPI.keys.create({
        ...common,
        user_email: values.user_email?.trim(),
      } satisfies ManagedKeyInput);
    },
    onSuccess: (result, variables) => {
      if (isManagementPendingResponse(result)) {
        setEditor(undefined);
        form.resetFields();
        message.warning(t('litellmops.management.pending'));
        return;
      }
      if (variables.key) {
        setEditor(undefined);
        form.resetFields();
        message.success(t('litellmops.common.saved'));
      } else {
        const outcome = managementMutationOutcome(result);
        revealSecret(outcome.status === 'completed' ? outcome.rawKey : undefined, () =>
          save.reset(),
        );
      }
    },
    onError: (error) => message.error(getRequestErrorMessage(error)),
    onSettled: refresh,
  });
  const act = useMutation({
    mutationFn: async ({ action, key }: { action: KeyAction; key: ManagedKey }) => {
      if (action === 'block') return operationsAPI.keys.block(key.id);
      if (action === 'unblock') return operationsAPI.keys.unblock(key.id);
      if (action === 'rotate') return operationsAPI.keys.rotate(key.id);
      if (action === 'reset-spend') return operationsAPI.keys.resetSpend(key.id);
      return operationsAPI.keys.remove(key.id);
    },
    onSuccess: (result, variables) => {
      if (isManagementPendingResponse(result)) {
        message.warning(t('litellmops.management.pending'));
        return;
      }
      if (variables.action === 'rotate') {
        const outcome = managementMutationOutcome(result);
        revealSecret(outcome.status === 'completed' ? outcome.rawKey : undefined, () =>
          act.reset(),
        );
        return;
      }
      message.success(
        t(
          variables.action === 'remove'
            ? 'litellmops.common.deleted'
            : 'litellmops.keys.actionDone',
        ),
      );
    },
    onError: (error) => message.error(getRequestErrorMessage(error)),
    onSettled: refresh,
  });

  if (!access.canReadKeys) return <PageForbidden />;

  const openEditor = (key?: ManagedKey) => {
    setEditor({ key });
    form.setFieldsValue(
      key
        ? {
            alias: key.alias ?? '',
            user_email: key.user_email,
            models: modelsOf(key.models),
            max_budget: key.max_budget ?? undefined,
            tpm_limit: key.tpm_limit ?? undefined,
            rpm_limit: key.rpm_limit ?? undefined,
            max_parallel_requests: key.max_parallel_requests ?? undefined,
            expires: key.expires ?? undefined,
          }
        : {
            alias: '',
            user_email: '',
            models: [],
            tpm_limit: 600_000,
          },
    );
  };

  const confirmAction = (action: KeyAction, key: ManagedKey) => {
    const dangerous = ['remove', 'rotate', 'reset-spend', 'block'].includes(action);
    modal.confirm({
      title: t(`litellmops.keys.action.${action}`),
      content: t(`litellmops.keys.warning.${action}`),
      okButtonProps: { danger: dangerous },
      onOk: () => act.mutateAsync({ action, key }),
    });
  };

  const columns: TableColumnsType<ManagedKey> = [
    {
      title: t('litellmops.keys.alias'),
      dataIndex: 'alias',
      fixed: 'left',
      width: 160,
      render: (value) => (value ? <Tag color="blue">{value}</Tag> : '—'),
    },
    {
      title: t('litellmops.keys.prefix'),
      dataIndex: 'key_hash_prefix',
      width: 150,
      render: (value) => <code>{value}…</code>,
    },
    {
      title: t('litellmops.keys.owner'),
      dataIndex: 'user_email',
      width: 220,
      render: (value) => value || '—',
    },
    {
      title: t('litellmops.keys.budget'),
      dataIndex: 'max_budget',
      width: 110,
      render: (value) => formatUsd(value),
    },
    {
      title: t('litellmops.keys.spend'),
      dataIndex: 'spend',
      width: 110,
      render: (value) => formatUsd(value, 4),
    },
    { title: t('litellmops.keys.tpm'), dataIndex: 'tpm_limit', width: 110, render: formatInteger },
    { title: t('litellmops.keys.rpm'), dataIndex: 'rpm_limit', width: 90, render: formatInteger },
    {
      title: t('litellmops.keys.expires'),
      dataIndex: 'expires',
      width: 180,
      render: formatDateTime,
    },
    {
      title: t('litellmops.keys.type'),
      dataIndex: 'is_session_key',
      width: 120,
      render: (session) => (
        <Tag color={session ? 'orange' : 'green'}>
          {t(session ? 'litellmops.keys.session' : 'litellmops.keys.api')}
        </Tag>
      ),
    },
    {
      title: t('litellmops.common.status'),
      dataIndex: 'blocked',
      width: 100,
      render: (blocked) => (
        <Tag color={blocked ? 'error' : 'success'}>
          {t(blocked ? 'litellmops.common.blocked' : 'litellmops.common.active')}
        </Tag>
      ),
    },
    {
      title: t('litellmops.common.actions'),
      key: 'actions',
      fixed: 'right',
      width: 360,
      render: (_, key) =>
        access.canWriteKeys || access.canRevokeKeys || access.canIssueKeys ? (
          <Space size={2} wrap>
            {access.canWriteKeys ? (
              <Button type="link" size="small" onClick={() => openEditor(key)}>
                {t('litellmops.common.edit')}
              </Button>
            ) : null}
            {access.canRevokeKeys ? (
              <Button
                size="small"
                danger={!key.blocked}
                onClick={() => confirmAction(key.blocked ? 'unblock' : 'block', key)}
              >
                {t(`litellmops.keys.action.${key.blocked ? 'unblock' : 'block'}`)}
              </Button>
            ) : null}
            {access.canIssueKeys && !key.is_session_key ? (
              <Button type="link" size="small" danger onClick={() => confirmAction('rotate', key)}>
                {t('litellmops.keys.action.rotate')}
              </Button>
            ) : null}
            {access.canWriteKeys ? (
              <Button type="link" size="small" onClick={() => confirmAction('reset-spend', key)}>
                {t('litellmops.keys.action.reset-spend')}
              </Button>
            ) : null}
            {access.canRevokeKeys ? (
              <Button type="link" size="small" danger onClick={() => confirmAction('remove', key)}>
                {t('litellmops.keys.action.remove')}
              </Button>
            ) : null}
          </Space>
        ) : null,
    },
  ];

  return (
    <PageContainer title={t('litellmops.keys.title')} content={t('litellmops.keys.description')}>
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Alert showIcon type="warning" message={t('litellmops.keys.securityPolicy')} />
        <Card
          extra={
            <Space wrap>
              <Input.Search
                allowClear
                placeholder={t('litellmops.keys.aliasSearch')}
                onSearch={(alias) =>
                  setParams((current) => ({ ...current, page: 1, alias: alias || undefined }))
                }
              />
              <Input.Search
                allowClear
                placeholder={t('litellmops.keys.ownerSearch')}
                onSearch={(user_email) =>
                  setParams((current) => ({
                    ...current,
                    page: 1,
                    user_email: user_email || undefined,
                  }))
                }
              />
              <Select
                allowClear
                placeholder={t('litellmops.keys.allTypes')}
                value={params.is_session_key}
                style={{ minWidth: 170 }}
                options={[
                  { value: 'false', label: t('litellmops.keys.apiOnly') },
                  { value: 'true', label: t('litellmops.keys.sessionOnly') },
                ]}
                onChange={(is_session_key) =>
                  setParams((current) => ({ ...current, page: 1, is_session_key }))
                }
              />
              {access.canIssueKeys ? (
                <Button type="primary" onClick={() => openEditor()}>
                  {t('litellmops.keys.issue')}
                </Button>
              ) : null}
            </Space>
          }
        >
          <OpsQueryState
            data={keys.data}
            error={keys.error}
            isError={keys.isError}
            isPending={keys.isPending}
            onRetry={() => void keys.refetch()}
          >
            <Table<ManagedKey>
              rowKey="id"
              loading={keys.isFetching}
              columns={columns}
              dataSource={keys.data?.items ?? []}
              locale={{ emptyText: <Empty description={t('litellmops.keys.empty')} /> }}
              scroll={{ x: 1_650 }}
              pagination={{
                current: params.page,
                pageSize: params.page_size,
                total: keys.data?.total,
                onChange: (page, pageSize) =>
                  setParams((current) => ({ ...current, page, page_size: pageSize })),
              }}
            />
          </OpsQueryState>
        </Card>
        <ManagementRecoveryPanel enabled={access.canResolveManagement} />
      </Space>

      <Modal
        destroyOnHidden
        title={t(editor?.key ? 'litellmops.keys.edit' : 'litellmops.keys.issue')}
        open={Boolean(editor)}
        confirmLoading={save.isPending}
        onCancel={() => setEditor(undefined)}
        onOk={() =>
          void form
            .validateFields()
            .then((values) => editor && save.mutate({ key: editor.key, values }))
        }
      >
        <Form form={form} layout="vertical">
          <Form.Item name="alias" label={t('litellmops.keys.alias')} rules={[{ required: true }]}>
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="user_email"
            label={t('litellmops.keys.owner')}
            rules={editor?.key ? [] : [{ required: true, type: 'email' }]}
          >
            <Input disabled={Boolean(editor?.key)} autoComplete="off" />
          </Form.Item>
          <Form.Item name="models" label={t('litellmops.keys.models')}>
            <Select mode="multiple" showSearch optionFilterProp="label" options={modelOptions} />
          </Form.Item>
          <Form.Item name="max_budget" label={t('litellmops.keys.budget')}>
            <InputNumber min={0} precision={2} style={{ width: '100%' }} />
          </Form.Item>
          <Space wrap>
            <Form.Item name="tpm_limit" label={t('litellmops.keys.tpm')}>
              <InputNumber min={0} precision={0} />
            </Form.Item>
            <Form.Item name="rpm_limit" label={t('litellmops.keys.rpm')}>
              <InputNumber min={0} precision={0} />
            </Form.Item>
            <Form.Item name="max_parallel_requests" label={t('litellmops.keys.parallel')}>
              <InputNumber min={0} precision={0} />
            </Form.Item>
          </Space>
          <Form.Item name="expires" label={t('litellmops.keys.expires')}>
            <Input placeholder="2027-01-01T00:00:00Z" autoComplete="off" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        destroyOnHidden
        closable={false}
        maskClosable={false}
        title={t('litellmops.keys.secretTitle')}
        open={Boolean(oneTimeKey)}
        okText={t('litellmops.keys.secretAcknowledge')}
        cancelButtonProps={{ style: { display: 'none' } }}
        onOk={() => setOneTimeKey(undefined)}
      >
        <Alert showIcon type="warning" message={t('litellmops.keys.secretWarning')} />
        <Typography.Paragraph
          copyable={{ text: oneTimeKey }}
          style={{ marginTop: 16, overflowWrap: 'anywhere' }}
        >
          <code>{oneTimeKey}</code>
        </Typography.Paragraph>
      </Modal>
    </PageContainer>
  );
}
